package graph_test

import (
	"path/filepath"
	"testing"

	"github.com/norlinga/trellis/internal/graph"
	"github.com/norlinga/trellis/internal/parser"
)

func fixtureDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", "valid"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return abs
}

// TestLoadFixtures exercises the whole pipeline (discover → parse →
// extract → resolve) against the three valid fixtures. It locks in the
// edge counts so a regression in handle extraction or matching shows up
// here before manifesting in a downstream tool.
func TestLoadFixtures(t *testing.T) {
	res, err := graph.Load(fixtureDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(res.ParseErrs) > 0 {
		t.Fatalf("parse errors: %v", res.ParseErrs)
	}
	g := res.Graph

	if got, want := len(g.Sidecars), 3; got != want {
		t.Fatalf("sidecars = %d, want %d", got, want)
	}

	// Expected edges:
	//   create_subscription.rb.trellis  →  payment_gateway.rb.trellis  (PaymentGateway.charge)
	//   subscription_analytics.rb.trellis → create_subscription.rb.trellis (Event:subscription.created)
	if got, want := len(g.Edges), 2; got != want {
		t.Fatalf("edges = %d, want %d:\n%+v", got, want, g.Edges)
	}

	// Expected unresolved:
	//   UserRecord (consumed by create_subscription, no provider)
	//   Event:subscription.cancelled (consumed by analytics, no provider)
	if got, want := len(g.Unresolved), 2; got != want {
		t.Fatalf("unresolved = %d, want %d:\n%+v", got, want, g.Unresolved)
	}
}

func TestDepsAndDependents(t *testing.T) {
	res, err := graph.Load(fixtureDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	g := res.Graph
	cs := findByBase(t, g, "create_subscription.rb.trellis")
	pg := findByBase(t, g, "payment_gateway.rb.trellis")
	an := findByBase(t, g, "subscription_analytics.rb.trellis")

	// create_subscription depends on payment_gateway.
	deps := g.Deps(cs)
	if len(deps) != 1 || deps[0].To != pg {
		t.Fatalf("create_subscription deps: want [→payment_gateway], got %+v", deps)
	}

	// payment_gateway has create_subscription as a dependent.
	dependents := g.Dependents(pg)
	if len(dependents) != 1 || dependents[0].From != cs {
		t.Fatalf("payment_gateway dependents: want [←create_subscription], got %+v", dependents)
	}

	// analytics depends on create_subscription via Event:subscription.created.
	deps = g.Deps(an)
	if len(deps) != 1 || deps[0].To != cs {
		t.Fatalf("analytics deps: want [→create_subscription], got %+v", deps)
	}
	if want := "Event:subscription.created"; deps[0].Handle.String() != want {
		t.Fatalf("analytics edge handle = %q, want %q", deps[0].Handle, want)
	}
}

// TestDownstream walks the chain payment_gateway → create_subscription →
// subscription_analytics. The blast-radius query from PaymentGateway must
// reach both downstream consumers; from analytics it must reach nobody.
func TestDownstream(t *testing.T) {
	res, err := graph.Load(fixtureDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	g := res.Graph
	pg := findByBase(t, g, "payment_gateway.rb.trellis")
	cs := findByBase(t, g, "create_subscription.rb.trellis")
	an := findByBase(t, g, "subscription_analytics.rb.trellis")

	got := g.Downstream(pg)
	if len(got) != 2 {
		t.Fatalf("Downstream(pg): want 2, got %d (%v)", len(got), pathsOf(got))
	}
	if !inSlice(pathsOf(got), cs.Path) || !inSlice(pathsOf(got), an.Path) {
		t.Fatalf("Downstream(pg) missing expected entries; got %v", pathsOf(got))
	}
	// BFS guarantee: direct dependents come before transitive ones.
	if got[0] != cs {
		t.Fatalf("Downstream(pg)[0] = %s, want create_subscription (closer dependent)", got[0].Path)
	}

	if got := g.Downstream(an); len(got) != 0 {
		t.Fatalf("Downstream(analytics): want empty, got %v", pathsOf(got))
	}
}

func inSlice(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func pathsOf(scs []*graph.Sidecar) []string {
	out := make([]string, len(scs))
	for i, sc := range scs {
		out[i] = sc.Path
	}
	return out
}

func TestHandleExtraction(t *testing.T) {
	res, err := graph.Load(fixtureDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cs := findByBase(t, res.Graph, "create_subscription.rb.trellis")

	// Sanity-check that both handle shapes survived extraction with the right
	// canonical form. This is the unit-level guard for decision #6.
	wantProvides := []string{"Subscription.create", "Event:subscription.created"}
	if got := handleStrings(cs.Provides); !equalSlices(got, wantProvides) {
		t.Fatalf("provides = %v, want %v", got, wantProvides)
	}
	wantConsumes := []string{"PaymentGateway.charge", "UserRecord"}
	if got := handleStrings(cs.Consumes); !equalSlices(got, wantConsumes) {
		t.Fatalf("consumes = %v, want %v", got, wantConsumes)
	}

	// Description for path-handle entries should preserve the rest of the
	// line (decision #6: opaque, never interpreted).
	if got := cs.Provides[0].Description; got == "" || !contains(got, "raises PaymentError") {
		t.Fatalf("provides[0] description = %q, want non-empty containing 'raises PaymentError'", got)
	}
}

func TestSourceAnchorExtraction(t *testing.T) {
	src := []byte(`Feature: Anchored
  "source anchor extraction"

  Provides:
    - Billing.Proration.calculate @source("line:42-68") -> Money
`)
	tree, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close()
	if tree.RootNode().HasError() {
		t.Fatalf("parse errors:\n%s", tree.RootNode().ToSexp())
	}
	sc, err := graph.Extract(tree, src, "billing.go.trellis")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(sc.Provides) != 1 {
		t.Fatalf("provides = %d, want 1", len(sc.Provides))
	}
	entry := sc.Provides[0]
	if got, want := entry.Handle.String(), "Billing.Proration.calculate"; got != want {
		t.Fatalf("handle = %q, want %q", got, want)
	}
	if entry.SourceAnchor == nil {
		t.Fatalf("SourceAnchor = nil")
	}
	if got, want := entry.SourceAnchor.Value, "line:42-68"; got != want {
		t.Fatalf("anchor value = %q, want %q", got, want)
	}
	if got, want := entry.SourceAnchor.StartLine, 42; got != want {
		t.Fatalf("StartLine = %d, want %d", got, want)
	}
	if got, want := entry.SourceAnchor.EndLine, 68; got != want {
		t.Fatalf("EndLine = %d, want %d", got, want)
	}
	if got := entry.Description; got != "-> Money" {
		t.Fatalf("description = %q, want %q", got, "-> Money")
	}
}

func findByBase(t *testing.T, g *graph.Graph, base string) *graph.Sidecar {
	t.Helper()
	for _, sc := range g.Sidecars {
		if filepath.Base(sc.Path) == base {
			return sc
		}
	}
	t.Fatalf("sidecar %q not found", base)
	return nil
}

func handleStrings(es []graph.Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Handle.String()
	}
	return out
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
