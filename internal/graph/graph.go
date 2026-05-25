package graph

import "sort"

// Graph is the resolved dependency graph over a workspace of sidecars.
//
// Edges are forward-only (consumer → producer); reverse lookups go through
// dependentsBy, which is built once at construction time. Callers should
// treat all maps and slices as read-only after Build returns; mutation will
// desync the indices.
type Graph struct {
	Sidecars   []*Sidecar
	byPath     map[string]*Sidecar
	providedBy map[Handle][]*Sidecar // who Provides this handle (multi: lint error, graph tolerates)

	Edges      []Edge
	depsBy     map[string][]Edge // sidecar.Path → outbound edges (its Consumes)
	dependents map[string][]Edge // sidecar.Path → inbound edges (sidecars that Consume something it Provides)

	Unresolved []UnresolvedConsume // Consumes handles with no matching Provides
}

// Edge is one resolved Consumes → Provides relationship.
type Edge struct {
	From   *Sidecar // consumer
	To     *Sidecar // producer
	Handle Handle   // the resolved handle
}

// UnresolvedConsume records a Consumes entry that resolved to no Provider.
// Graph construction does not treat this as a fatal error — surface it as
// a diagnostic instead. The whitepaper §4.1 broken-link check is a linter
// rule that consumes this list.
type UnresolvedConsume struct {
	From   *Sidecar
	Handle Handle
}

// Build assembles a Graph from a slice of Sidecars. Caller retains
// ownership of the input slice; Build does not copy Sidecar values.
//
// Resolution semantics:
//   - A Consumes handle resolves to every Sidecar whose Provides list
//     contains an equal Handle. Multiple resolutions create multiple edges.
//   - Self-edges (a Sidecar providing and consuming the same handle) are
//     emitted; the linter decides whether to warn.
//   - A Consumes handle with no matching Provides becomes an
//     UnresolvedConsume entry, not an edge.
func Build(sidecars []*Sidecar) *Graph {
	g := &Graph{
		Sidecars:   sidecars,
		byPath:     make(map[string]*Sidecar, len(sidecars)),
		providedBy: make(map[Handle][]*Sidecar),
		depsBy:     make(map[string][]Edge),
		dependents: make(map[string][]Edge),
	}
	for _, sc := range sidecars {
		g.byPath[sc.Path] = sc
		for _, p := range sc.Provides {
			g.providedBy[p.Handle] = append(g.providedBy[p.Handle], sc)
		}
	}
	for _, sc := range sidecars {
		for _, c := range sc.Consumes {
			producers := g.providedBy[c.Handle]
			if len(producers) == 0 {
				g.Unresolved = append(g.Unresolved, UnresolvedConsume{From: sc, Handle: c.Handle})
				continue
			}
			for _, p := range producers {
				edge := Edge{From: sc, To: p, Handle: c.Handle}
				g.Edges = append(g.Edges, edge)
				g.depsBy[sc.Path] = append(g.depsBy[sc.Path], edge)
				g.dependents[p.Path] = append(g.dependents[p.Path], edge)
			}
		}
	}
	return g
}

// Lookup returns the Sidecar at path, or nil if the path was not in the
// input set.
func (g *Graph) Lookup(path string) *Sidecar { return g.byPath[path] }

// ProvidersOf returns every sidecar that provides h. A handle should normally
// have exactly one provider; multiple providers are tolerated here so callers
// can surface duplicate-provider diagnostics deliberately.
func (g *Graph) ProvidersOf(h Handle) []*Sidecar {
	providers := g.providedBy[h]
	if len(providers) == 0 {
		return nil
	}
	out := make([]*Sidecar, len(providers))
	copy(out, providers)
	return out
}

// Deps returns the outbound edges from sc (what it consumes that resolved
// to a producer). Returns nil if sc is not in the graph or has no
// resolvable consumes.
func (g *Graph) Deps(sc *Sidecar) []Edge {
	if sc == nil {
		return nil
	}
	return g.depsBy[sc.Path]
}

// Dependents returns the inbound edges to sc (sidecars that consume
// something sc provides).
func (g *Graph) Dependents(sc *Sidecar) []Edge {
	if sc == nil {
		return nil
	}
	return g.dependents[sc.Path]
}

// Orphans returns Sidecars whose Provides list contains at least one
// handle but whose Provides have zero inbound edges (no consumer). An empty
// Provides list does not count as orphaned — that's a different lint
// signal.
//
// Whitepaper §4.1 mentions both "orphaned sidecars" (source file gone) and
// "Provides: unreferenced." This implements the second; the first requires
// touching the filesystem and lives at a layer above the graph.
func (g *Graph) Orphans() []*Sidecar {
	var out []*Sidecar
	for _, sc := range g.Sidecars {
		if len(sc.Provides) == 0 {
			continue
		}
		if len(g.dependents[sc.Path]) == 0 {
			out = append(out, sc)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Downstream returns every Sidecar that transitively depends on sc — the
// "blast radius" question from whitepaper §4.1: *what would break if I
// changed this signature?* sc is excluded from the result.
//
// Ordering: BFS, closer dependents first. Within a level, the order
// follows insertion order in the underlying dependents slice (deterministic
// because edge construction order is deterministic). Cycles are tolerated
// via the visited set; the result is finite.
func (g *Graph) Downstream(sc *Sidecar) []*Sidecar {
	if sc == nil {
		return nil
	}
	visited := map[*Sidecar]bool{sc: true}
	var out []*Sidecar
	queue := []*Sidecar{sc}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range g.dependents[cur.Path] {
			if visited[e.From] {
				continue
			}
			visited[e.From] = true
			out = append(out, e.From)
			queue = append(queue, e.From)
		}
	}
	return out
}

// DuplicateProvides returns handles that appear in the Provides list of
// more than one Sidecar. The whitepaper §4.4 flags this as "duplicate
// features at write time" — handle-equality only here, fuzzy/semantic
// matching is explicitly deferred (decision #6).
func (g *Graph) DuplicateProvides() map[Handle][]*Sidecar {
	out := make(map[Handle][]*Sidecar)
	for h, scs := range g.providedBy {
		if len(scs) > 1 {
			out[h] = scs
		}
	}
	return out
}
