package lint

import (
	"fmt"
	"path/filepath"

	"github.com/norlinga/trellis/internal/graph"
)

// ---------------------------------------------------------------------
// Layer enforcement
// ---------------------------------------------------------------------

// LayerViolation flags Consumes edges that violate `MUST NOT` rules in
// the `layer_dependencies:` section of the active policy.
//
// Resolution:
//   - Source layer comes from the consuming sidecar's `@layer:` value.
//   - Target layer comes from the producing sidecar's `@layer:` value.
//   - If either side lacks `@layer:`, the edge is skipped. A sidecar
//     without a layer is "not part of any layer constraint" — it
//     participates in the graph but not in this rule.
//   - Each edge is checked against every MUST NOT rule independently;
//     two rules that both apply will both fire.
//
// The diagnostic is anchored to the consuming sidecar at the
// `handle_entry` for the consumed handle, so editor jumps land on the
// offending bullet line.
type LayerViolation struct {
	Policy *Policy
}

func (*LayerViolation) Code() string { return "policy-layer-violation" }

func (r *LayerViolation) Check(files []*File, g *graph.Graph) []Diagnostic {
	if r == nil || r.Policy == nil || g == nil || len(r.Policy.LayerRules) == 0 {
		return nil
	}
	return checkPolicyEdges(r.Policy.LayerRules, "layer", "@layer", "policy-layer-violation", files, g)
}

// ---------------------------------------------------------------------
// Stability enforcement
// ---------------------------------------------------------------------

// StabilityViolation flags Consumes edges that violate `MUST NOT` rules
// in the `stability_tiers:` section. Wiring is identical to
// LayerViolation but reads `@stability:` instead.
//
// The canonical rule this is built around is the whitepaper's
// `stable MUST NOT consume experimental` — preventing the load-bearing
// codepath from depending on something explicitly marked unstable.
type StabilityViolation struct {
	Policy *Policy
}

func (*StabilityViolation) Code() string { return "policy-stability-violation" }

func (r *StabilityViolation) Check(files []*File, g *graph.Graph) []Diagnostic {
	if r == nil || r.Policy == nil || g == nil || len(r.Policy.StabilityRules) == 0 {
		return nil
	}
	return checkPolicyEdges(r.Policy.StabilityRules, "stability", "@stability", "policy-stability-violation", files, g)
}

// ---------------------------------------------------------------------
// Shared edge-walker
// ---------------------------------------------------------------------

// checkPolicyEdges is the body of both LayerViolation and
// StabilityViolation. The two rules differ only in which rule slice they
// consult and which frontmatter key carries the dimension's label.
//
// dimension is a short label used in the diagnostic message ("layer",
// "stability") so messages read naturally regardless of which rule type
// fired.
func checkPolicyEdges(rules []PolicyRule, dimension, frontmatterKey, code string, files []*File, g *graph.Graph) []Diagnostic {
	fileBy := indexFilesByPath(files)
	locs := buildHandleLocations(files)

	var out []Diagnostic
	for _, e := range g.Edges {
		srcFile := fileBy[e.From.Path]
		dstFile := fileBy[e.To.Path]
		srcLabel := srcFile.FrontmatterValue(frontmatterKey)
		dstLabel := dstFile.FrontmatterValue(frontmatterKey)
		if srcLabel == "" || dstLabel == "" {
			continue
		}
		for _, rule := range rules {
			if rule.Verb != VerbMustNot {
				continue
			}
			if rule.Source != srcLabel || rule.Target != dstLabel {
				continue
			}
			rng := FileStart()
			if r, ok := locs[handleSiteKey{Path: e.From.Path, Handle: e.Handle}]; ok {
				rng = r
			}
			out = append(out, Diagnostic{
				Code:     code,
				Severity: SeverityError,
				Message: fmt.Sprintf(
					"%s policy violation: %s (%s=%s) MUST NOT consume %s (%s=%s) — consumed handle: %s [from %s:%d]",
					dimension,
					filepath.Base(e.From.Path), dimension, srcLabel,
					filepath.Base(e.To.Path), dimension, dstLabel,
					e.Handle,
					filepath.Base(rule.From), rule.Line,
				),
				Path:  e.From.Path,
				Range: rng,
			})
		}
	}
	return out
}

// indexFilesByPath returns a path → File map for O(1) lookups during
// edge iteration. The graph stores Sidecar pointers, not File pointers,
// so we can't go directly from an edge to a File — this map bridges them.
func indexFilesByPath(files []*File) map[string]*File {
	out := make(map[string]*File, len(files))
	for _, f := range files {
		out[f.Path] = f
	}
	return out
}
