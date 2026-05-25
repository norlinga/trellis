package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/norlinga/trellis/internal/cli"
)

// run drives the trellis cobra root in-process. It returns stdout, the
// concatenation of stderr (cobra writes errors here when --help triggers
// nothing else), and the error the root returned. Tests assert on whichever
// of the three is meaningful.
func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := cli.NewRootCmd()
	var so, se bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&so)
	cmd.SetErr(&se)
	err = cmd.Execute()
	return so.String(), se.String(), err
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Minimal valid sidecar bodies. Two-segment handles so we can wire edges
// without dragging in a long fixture.
const (
	provFoo = `Feature: A
  "provides Foo"

  Provides:
    - Foo
`
	consFoo = `Feature: B
  "consumes Foo"

  Consumes:
    - Foo
`
	provFooConsBar = `Feature: C
  "provides Foo, consumes Bar"

  Provides:
    - Foo

  Consumes:
    - Bar
`
	provBar = `Feature: D
  "provides Bar"

  Provides:
    - Bar
`
)

// ---------------------------------------------------------------------
// Help / shape
// ---------------------------------------------------------------------

func TestRootHelpListsGraphSubcommand(t *testing.T) {
	out, _, err := run(t, "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}
	if !strings.Contains(out, "graph") {
		t.Fatalf("--help missing 'graph' subcommand:\n%s", out)
	}
	if !strings.Contains(out, "locate") {
		t.Fatalf("--help missing 'locate' subcommand:\n%s", out)
	}
}

func TestGraphHelpListsAllSubcommands(t *testing.T) {
	out, _, err := run(t, "graph", "--help")
	if err != nil {
		t.Fatalf("graph --help: %v", err)
	}
	for _, want := range []string{"build", "deps", "dependents", "downstream", "orphans", "parse"} {
		if !strings.Contains(out, want) {
			t.Errorf("graph --help missing %q:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------
// graph build
// ---------------------------------------------------------------------

func TestGraphBuildSummary(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go.trellis"), provFoo)
	writeFile(t, filepath.Join(dir, "b.go.trellis"), consFoo)

	out, _, err := run(t, "graph", "build", dir)
	if err != nil {
		t.Fatalf("graph build: %v", err)
	}
	for _, want := range []string{"sidecars:        2", "resolved edges:  1", "unresolved:      0"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestGraphBuildReportsParseErrorsAndContinues(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "good.go.trellis"), provFoo)
	// A file that the parser will reject — no Feature line at all.
	writeFile(t, filepath.Join(dir, "bad.go.trellis"), "this is not a sidecar\n")

	out, _, err := run(t, "graph", "build", dir)
	if err != nil {
		t.Fatalf("graph build: %v", err)
	}
	if !strings.Contains(out, "sidecars:        1") {
		t.Errorf("good sidecar should still load; got:\n%s", out)
	}
	if !strings.Contains(out, "parse errors:") || !strings.Contains(out, "bad.go.trellis") {
		t.Errorf("parse error not surfaced for bad.go.trellis; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------
// graph parse
// ---------------------------------------------------------------------

func TestGraphParseHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go.trellis")
	writeFile(t, path, provFoo)

	out, _, err := run(t, "graph", "parse", path)
	if err != nil {
		t.Fatalf("graph parse: %v", err)
	}
	if !strings.Contains(out, "source_file") {
		t.Errorf("parse output missing root S-expression marker:\n%s", out)
	}
}

func TestGraphParseRejectsErrorTree(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.go.trellis")
	writeFile(t, path, "garbage that doesn't start with a Feature\n")

	_, _, err := run(t, "graph", "parse", path)
	if err == nil {
		t.Fatalf("expected an error from graph parse on a malformed file")
	}
	if !strings.Contains(err.Error(), "parse errors") {
		t.Errorf("error message should mention parse errors; got %v", err)
	}
}

// ---------------------------------------------------------------------
// graph deps / dependents / downstream / orphans
// ---------------------------------------------------------------------

func TestGraphDepsResolvedAndUnresolved(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "p.go.trellis"), provFoo)
	writeFile(t, filepath.Join(dir, "c.go.trellis"), `Feature: C
  "consumes Foo and Missing"

  Consumes:
    - Foo
    - Missing.thing
`)

	out, _, err := run(t, "graph", "deps", "c.go.trellis", dir)
	if err != nil {
		t.Fatalf("graph deps: %v", err)
	}
	if !strings.Contains(out, "Foo →") || !strings.Contains(out, "p.go.trellis") {
		t.Errorf("expected resolved edge to p.go.trellis:\n%s", out)
	}
	if !strings.Contains(out, "Missing.thing → (unresolved)") {
		t.Errorf("expected unresolved Missing.thing:\n%s", out)
	}
}

func TestGraphDependents(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "p.go.trellis"), provFoo)
	writeFile(t, filepath.Join(dir, "c.go.trellis"), consFoo)

	out, _, err := run(t, "graph", "dependents", "p.go.trellis", dir)
	if err != nil {
		t.Fatalf("graph dependents: %v", err)
	}
	if !strings.Contains(out, "Foo ←") || !strings.Contains(out, "c.go.trellis") {
		t.Errorf("expected reverse edge from c.go.trellis:\n%s", out)
	}
}

func TestGraphDownstreamTransitive(t *testing.T) {
	// A chain: a provides Foo → b consumes Foo, b provides Bar → c consumes Bar.
	// Downstream(a) should include both b and c, with b first (BFS).
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go.trellis"), provFoo)
	writeFile(t, filepath.Join(dir, "b.go.trellis"), `Feature: B
  "consumes Foo, provides Bar"

  Provides:
    - Bar

  Consumes:
    - Foo
`)
	writeFile(t, filepath.Join(dir, "c.go.trellis"), `Feature: C
  "consumes Bar"

  Consumes:
    - Bar
`)

	out, _, err := run(t, "graph", "downstream", "a.go.trellis", dir)
	if err != nil {
		t.Fatalf("graph downstream: %v", err)
	}
	lines := splitLines(out)
	if len(lines) != 2 {
		t.Fatalf("expected 2 downstream entries, got %d:\n%s", len(lines), out)
	}
	// BFS: direct dependent first.
	if !strings.HasSuffix(lines[0], "b.go.trellis") {
		t.Errorf("first downstream should be b.go.trellis (closer dependent); got %q", lines[0])
	}
	if !strings.HasSuffix(lines[1], "c.go.trellis") {
		t.Errorf("second downstream should be c.go.trellis; got %q", lines[1])
	}
}

func TestGraphOrphans(t *testing.T) {
	// p provides Foo, no one consumes; q provides Bar, r consumes Bar.
	// Only p should be listed as orphan.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "p.go.trellis"), provFoo)
	writeFile(t, filepath.Join(dir, "q.go.trellis"), provBar)
	writeFile(t, filepath.Join(dir, "r.go.trellis"), `Feature: R
  "consumes Bar"

  Consumes:
    - Bar
`)

	out, _, err := run(t, "graph", "orphans", dir)
	if err != nil {
		t.Fatalf("graph orphans: %v", err)
	}
	lines := splitLines(out)
	if len(lines) != 1 || !strings.HasSuffix(lines[0], "p.go.trellis") {
		t.Fatalf("expected exactly p.go.trellis as orphan, got: %v", lines)
	}
}

// ---------------------------------------------------------------------
// locate
// ---------------------------------------------------------------------

func TestLocateJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "example.cbl"), "       IDENTIFICATION DIVISION.\n")
	writeFile(t, filepath.Join(dir, "example.cbl.trellis"), `Feature: Example Workflow
  "generic decision"

  Provides:
    - Decision:ExampleWorkflow.rule @source("label:DECISION-PARAGRAPH") determines outcome
`)

	out, _, err := run(t, "locate", "Decision:ExampleWorkflow.rule", "--json", dir)
	if err != nil {
		t.Fatalf("locate: %v\n%s", err, out)
	}
	for _, want := range []string{
		`"handle": "Decision:ExampleWorkflow.rule"`,
		`"source_path": "` + filepath.Join(dir, "example.cbl") + `"`,
		`"value": "label:DECISION-PARAGRAPH"`,
		`"kind": "label"`,
		`"target": "DECISION-PARAGRAPH"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("locate JSON missing %q:\n%s", want, out)
		}
	}
}

func TestLocateErrorsWhenProviderHasNoAnchor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "example.cbl.trellis"), `Feature: Example Workflow
  "generic decision"

  Provides:
    - Decision:ExampleWorkflow.rule
`)

	_, _, err := run(t, "locate", "Decision:ExampleWorkflow.rule", dir)
	if err == nil {
		t.Fatalf("expected missing-anchor error")
	}
	if !strings.Contains(err.Error(), "no source anchor") {
		t.Fatalf("error = %v, want no source anchor", err)
	}
}

// ---------------------------------------------------------------------
// Lookup ergonomics
// ---------------------------------------------------------------------

func TestGraphDepsErrorsOnAmbiguousBasename(t *testing.T) {
	dir := t.TempDir()
	// Same basename in two subdirectories — basename lookup must refuse.
	writeFile(t, filepath.Join(dir, "x", "shared.go.trellis"), provFoo)
	writeFile(t, filepath.Join(dir, "y", "shared.go.trellis"), provBar)

	_, _, err := run(t, "graph", "deps", "shared.go.trellis", dir)
	if err == nil {
		t.Fatalf("expected ambiguous-basename error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error should mention 'ambiguous'; got %v", err)
	}
}

func TestGraphDepsErrorsOnUnknownTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "p.go.trellis"), provFoo)

	_, _, err := run(t, "graph", "deps", "nope.go.trellis", dir)
	if err == nil {
		t.Fatalf("expected error for unknown target")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found'; got %v", err)
	}
}

// ---------------------------------------------------------------------
// trellis lint
// ---------------------------------------------------------------------

func TestLintFlagsExpectedRulesAndExitsNonZeroOnErrors(t *testing.T) {
	dir := t.TempDir()
	// One sidecar deliberately rigged to fire the design rules:
	//   - missing @reviewed (warn: frontmatter-missing-required)
	//   - alias kind 'happy' (warn: scenario-kind-canonical)
	//   - no negative scenario (warn)
	//   - no Invariants (warn)
	//   - high consumes count (error)
	writeFile(t, filepath.Join(dir, "noisy.go.trellis"), `@owner: T
@stability: stable
@since: 2026-01-01

Feature: F
  "deliberately noisy"

  Consumes:
    - h.a
    - h.b
    - h.c
    - h.d
    - h.e
    - h.f
    - h.g
    - h.h
    - h.i

  Scenario (happy): h
    Given x
`)

	out, _, err := run(t, "lint", dir)
	if err == nil {
		t.Fatalf("expected non-zero exit on lint errors")
	}
	for _, want := range []string{
		"frontmatter-missing-required",
		"scenario-kind-canonical",
		"missing-negative-scenario",
		"missing-invariants",
		"consumes-count",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing rule code %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "1 error(s)") {
		t.Errorf("expected '1 error(s)' summary line:\n%s", out)
	}
}

func TestLintReportsStaleReviewed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old.go"), "package old\n")
	// Year-old @reviewed; rule fires at 180 days warn / 365 days error.
	// Using a date well in the past keeps the test stable as time advances.
	writeFile(t, filepath.Join(dir, "old.go.trellis"), `@owner: T
@stability: stable
@since: 2020-01-01
@reviewed: 2020-01-01

Feature: F
  "ancient"

  Provides:
    - F.do

  Invariants:
    - it MUST stay true

  Scenario (happy-path): h
    Given x

  Scenario (negative): n
    Given y
`)
	out, _, err := run(t, "lint", dir)
	if err == nil {
		t.Fatalf("expected non-zero exit on stale-reviewed error")
	}
	if !strings.Contains(out, "stale-reviewed") {
		t.Errorf("output missing 'stale-reviewed' code:\n%s", out)
	}
}

func TestLintCleanWorkspace(t *testing.T) {
	dir := t.TempDir()
	// Write the paired source file too so OrphanSourceFile is silent.
	writeFile(t, filepath.Join(dir, "good.go"), "package good\n")
	writeFile(t, filepath.Join(dir, "good.go.trellis"), `@owner: T
@stability: stable
@since: 2026-01-01
@reviewed: 2026-05-10

Feature: F
  "fully populated"

  Provides:
    - F.do

  Invariants:
    - The thing MUST stay true

  Scenario (happy-path): h
    Given x

  Scenario (negative): n
    Given y
`)
	out, _, err := run(t, "lint", dir)
	if err != nil {
		t.Fatalf("clean workspace: lint returned err=%v\n%s", err, out)
	}
	if !strings.Contains(out, "0 error(s), 0 warning(s)") {
		t.Errorf("expected zero-zero summary:\n%s", out)
	}
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
