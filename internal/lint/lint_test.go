package lint_test

import (
	"strings"
	"testing"
	"time"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/norlinga/trellis/internal/graph"
	"github.com/norlinga/trellis/internal/lint"
	"github.com/norlinga/trellis/internal/parser"
)

// parseFixture builds a *lint.File from an inline source string. The Tree
// is registered for cleanup so tests don't leak C memory.
func parseFixture(t *testing.T, src string) *lint.File {
	t.Helper()
	tree, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parser.Parse: %v", err)
	}
	t.Cleanup(func() { tree.Close() })
	if tree.RootNode().HasError() {
		t.Fatalf("fixture has parse errors:\n%s\n%s", src, tree.RootNode().ToSexp())
	}
	sc, err := graph.Extract(tree, []byte(src), "test.trellis")
	if err != nil {
		t.Fatalf("graph.Extract: %v", err)
	}
	// Re-parse via LoadWorkspace path to get override extraction.
	// The standalone test fixtures don't go through LoadWorkspace, so we
	// extract overrides here directly using the same logic — keeps test
	// shape parallel to production.
	return &lint.File{
		Path:      "test.trellis",
		Source:    []byte(src),
		Tree:      tree,
		Sidecar:   sc,
		Overrides: extractOverridesForTest(tree, []byte(src)),
	}
}

// extractOverridesForTest mirrors the linter's internal frontmatter walk.
// Kept separate from production code because the production helper is
// unexported (intentionally — the public API is HasOverride).
func extractOverridesForTest(tree *sitter.Tree, src []byte) map[string]string {
	out := map[string]string{}
	root := tree.RootNode()
	fm := root.ChildByFieldName("frontmatter")
	if fm == nil {
		return out
	}
	for i := uint(0); i < fm.NamedChildCount(); i++ {
		entry := fm.NamedChild(i)
		if entry.Kind() != "frontmatter_entry" {
			continue
		}
		keyNode := entry.ChildByFieldName("key")
		if keyNode == nil {
			continue
		}
		key := string(src[keyNode.StartByte():keyNode.EndByte()])
		if !strings.HasPrefix(key, lint.OverridePrefix) {
			continue
		}
		valNode := entry.ChildByFieldName("value")
		if valNode == nil || valNode.Kind() != "quoted_string" {
			continue
		}
		raw := string(src[valNode.StartByte():valNode.EndByte()])
		if len(raw) >= 2 {
			raw = raw[1 : len(raw)-1]
		}
		out[key] = raw
	}
	return out
}

// codes pulls just the diagnostic codes for terse assertions.
func codes(diags []lint.Diagnostic) []string {
	out := make([]string, len(diags))
	for i, d := range diags {
		out[i] = d.Code
	}
	return out
}

// ---------------------------------------------------------------------
// FrontmatterRequiredKeys
// ---------------------------------------------------------------------

func TestFrontmatterRequiredKeys_AllPresent(t *testing.T) {
	f := parseFixture(t, `@owner: Team
@stability: stable
@since: 2026-01-01
@reviewed: 2026-05-10

Feature: F
  "ok"
`)
	diags := (&lint.FrontmatterRequiredKeys{}).Check(f)
	if len(diags) != 0 {
		t.Fatalf("want 0 diags, got %v", codes(diags))
	}
}

func TestFrontmatterRequiredKeys_MissingTwo(t *testing.T) {
	f := parseFixture(t, `@owner: Team
@stability: stable

Feature: F
  "missing @since and @reviewed"
`)
	diags := (&lint.FrontmatterRequiredKeys{}).Check(f)
	if len(diags) != 2 {
		t.Fatalf("want 2 diags, got %d: %v", len(diags), diags)
	}
	for _, d := range diags {
		if d.Severity != lint.SeverityWarning {
			t.Errorf("expected warning severity, got %v", d.Severity)
		}
		if !strings.HasPrefix(d.Message, "missing required frontmatter key:") {
			t.Errorf("unexpected message: %q", d.Message)
		}
	}
}

// ---------------------------------------------------------------------
// ScenarioKindCanonical
// ---------------------------------------------------------------------

func TestScenarioKindCanonical_Canonical(t *testing.T) {
	f := parseFixture(t, `Feature: F
  "ok"

  Scenario (happy-path): nominal
    Given a thing
`)
	diags := (&lint.ScenarioKindCanonical{}).Check(f)
	if len(diags) != 0 {
		t.Fatalf("want 0 diags, got %v", codes(diags))
	}
}

func TestScenarioKindCanonical_AliasSuggestsCanonical(t *testing.T) {
	f := parseFixture(t, `Feature: F
  "ok"

  Scenario (happy): nominal
    Given a thing
`)
	diags := (&lint.ScenarioKindCanonical{}).Check(f)
	if len(diags) != 1 {
		t.Fatalf("want 1 diag, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "alias") || !strings.Contains(diags[0].Message, "happy-path") {
		t.Errorf("expected alias-suggests-canonical message; got %q", diags[0].Message)
	}
}

func TestScenarioKindCanonical_UnknownKind(t *testing.T) {
	f := parseFixture(t, `Feature: F
  "ok"

  Scenario (unicorn): mystery
    Given a thing
`)
	diags := (&lint.ScenarioKindCanonical{}).Check(f)
	if len(diags) != 1 {
		t.Fatalf("want 1 diag, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "not a canonical") {
		t.Errorf("expected not-canonical message; got %q", diags[0].Message)
	}
}

// ---------------------------------------------------------------------
// ScenarioCount
// ---------------------------------------------------------------------

func TestScenarioCount_BelowThreshold(t *testing.T) {
	f := parseFixture(t, makeScenarios(5))
	diags := (&lint.ScenarioCount{Warn: 10, Error: 15}).Check(f)
	if len(diags) != 0 {
		t.Fatalf("want 0 diags, got %v", codes(diags))
	}
}

func TestScenarioCount_WarnThreshold(t *testing.T) {
	f := parseFixture(t, makeScenarios(11))
	diags := (&lint.ScenarioCount{Warn: 10, Error: 15}).Check(f)
	if len(diags) != 1 || diags[0].Severity != lint.SeverityWarning {
		t.Fatalf("want 1 warning, got %v", diags)
	}
}

func TestScenarioCount_ErrorThreshold(t *testing.T) {
	f := parseFixture(t, makeScenarios(16))
	diags := (&lint.ScenarioCount{Warn: 10, Error: 15}).Check(f)
	if len(diags) != 1 || diags[0].Severity != lint.SeverityError {
		t.Fatalf("want 1 error, got %v", diags)
	}
}

func makeScenarios(n int) string {
	var b strings.Builder
	b.WriteString(`Feature: F
  "x"

`)
	for i := 0; i < n; i++ {
		b.WriteString("  Scenario (happy-path): s\n    Given a thing\n\n")
	}
	return b.String()
}

// ---------------------------------------------------------------------
// ConsumesCount
// ---------------------------------------------------------------------

func TestConsumesCount_BelowAndAboveThreshold(t *testing.T) {
	below := parseFixture(t, makeConsumes(3))
	above := parseFixture(t, makeConsumes(9))

	if got := (&lint.ConsumesCount{Warn: 5, Error: 8}).Check(below); len(got) != 0 {
		t.Fatalf("below threshold: want 0, got %v", codes(got))
	}
	got := (&lint.ConsumesCount{Warn: 5, Error: 8}).Check(above)
	if len(got) != 1 || got[0].Severity != lint.SeverityError {
		t.Fatalf("above error threshold: want 1 error, got %v", got)
	}
}

func makeConsumes(n int) string {
	var b strings.Builder
	b.WriteString(`Feature: F
  "x"

  Consumes:
`)
	for i := 0; i < n; i++ {
		b.WriteString("    - h.")
		// Tree-sitter rejects empty identifier paths, so generate distinct
		// 2-segment handles.
		b.WriteString("seg")
		b.WriteString(string(rune('a' + i)))
		b.WriteByte('\n')
	}
	return b.String()
}

// ---------------------------------------------------------------------
// MissingNegativeScenario
// ---------------------------------------------------------------------

func TestMissingNegativeScenario(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		wantDiag bool
	}{
		{"has negative", `Feature: F
  "x"

  Scenario (happy-path): h
    Given x

  Scenario (negative): n
    Given y
`, false},
		{"only happy-path", `Feature: F
  "x"

  Scenario (happy-path): h
    Given x
`, true},
		{"no scenarios at all", `Feature: F
  "x"
`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := parseFixture(t, tc.src)
			diags := (&lint.MissingNegativeScenario{}).Check(f)
			if got := len(diags) > 0; got != tc.wantDiag {
				t.Errorf("wantDiag=%v, got %v (%v)", tc.wantDiag, got, diags)
			}
		})
	}
}

// ---------------------------------------------------------------------
// MissingInvariants
// ---------------------------------------------------------------------

func TestMissingInvariants(t *testing.T) {
	with := parseFixture(t, `Feature: F
  "x"

  Invariants:
    - x MUST stay true
`)
	without := parseFixture(t, `Feature: F
  "x"

  Provides:
    - Foo
`)
	if got := (&lint.MissingInvariants{}).Check(with); len(got) != 0 {
		t.Errorf("with invariants: want 0, got %v", codes(got))
	}
	if got := (&lint.MissingInvariants{}).Check(without); len(got) != 1 {
		t.Errorf("without invariants: want 1, got %v", codes(got))
	}
}

// ---------------------------------------------------------------------
// Coordinator + format helpers
// ---------------------------------------------------------------------

func TestLinter_DefaultFiresExpectedRules(t *testing.T) {
	// A sidecar that violates several rules at once: missing @reviewed,
	// missing Invariants, missing negative scenario, alias kind.
	f := parseFixture(t, `@owner: T
@stability: stable
@since: 2026-01-01

Feature: F
  "deliberately deficient"

  Scenario (happy): h
    Given x
`)
	g := graph.Build([]*graph.Sidecar{f.Sidecar})
	diags := lint.Default().Lint([]*lint.File{f}, g)
	want := map[string]bool{
		"frontmatter-missing-required": true, // @reviewed
		"scenario-kind-canonical":      true, // 'happy' alias
		"missing-negative-scenario":    true,
		"missing-invariants":           true,
	}
	got := make(map[string]bool, len(diags))
	for _, d := range diags {
		got[d.Code] = true
	}
	for code := range want {
		if !got[code] {
			t.Errorf("expected diagnostic with code %q, got %v", code, codes(diags))
		}
	}
}

// ---------------------------------------------------------------------
// StaleReviewed
// ---------------------------------------------------------------------

func TestStaleReviewed_Recent(t *testing.T) {
	f := parseFixture(t, `@reviewed: 2026-04-01

Feature: F
  "x"
`)
	r := &lint.StaleReviewed{
		Now:        timeOn(2026, 5, 10), // ~40 days later
		WarnAfter:  180 * 24 * time.Hour,
		ErrorAfter: 365 * 24 * time.Hour,
	}
	if got := r.Check(f); len(got) != 0 {
		t.Errorf("recent date should not fire; got %v", got)
	}
}

func TestStaleReviewed_PastWarnThreshold(t *testing.T) {
	f := parseFixture(t, `@reviewed: 2025-10-01

Feature: F
  "x"
`)
	r := &lint.StaleReviewed{
		Now:        timeOn(2026, 5, 10), // ~221 days later, past 180-day warn
		WarnAfter:  180 * 24 * time.Hour,
		ErrorAfter: 365 * 24 * time.Hour,
	}
	got := r.Check(f)
	if len(got) != 1 || got[0].Severity != lint.SeverityWarning {
		t.Fatalf("want 1 warning, got %v", got)
	}
}

func TestStaleReviewed_PastErrorThreshold(t *testing.T) {
	f := parseFixture(t, `@reviewed: 2024-12-01

Feature: F
  "x"
`)
	r := &lint.StaleReviewed{
		Now:        timeOn(2026, 5, 10), // ~525 days later, past 365-day error
		WarnAfter:  180 * 24 * time.Hour,
		ErrorAfter: 365 * 24 * time.Hour,
	}
	got := r.Check(f)
	if len(got) != 1 || got[0].Severity != lint.SeverityError {
		t.Fatalf("want 1 error, got %v", got)
	}
	if !strings.Contains(got[0].Message, "525") && !strings.Contains(got[0].Message, "526") {
		t.Errorf("message should report age in days; got %q", got[0].Message)
	}
}

func TestStaleReviewed_NoReviewedKey(t *testing.T) {
	f := parseFixture(t, `Feature: F
  "x"
`)
	r := &lint.StaleReviewed{
		Now:        timeOn(2026, 5, 10),
		WarnAfter:  180 * 24 * time.Hour,
		ErrorAfter: 365 * 24 * time.Hour,
	}
	if got := r.Check(f); len(got) != 0 {
		t.Errorf("no @reviewed should not fire (different rule fires); got %v", got)
	}
}

func TestStaleReviewed_AnchorsToDateValue(t *testing.T) {
	f := parseFixture(t, `@reviewed: 2024-01-01

Feature: F
  "x"
`)
	r := &lint.StaleReviewed{
		Now:        timeOn(2026, 5, 10),
		WarnAfter:  180 * 24 * time.Hour,
		ErrorAfter: 365 * 24 * time.Hour,
	}
	got := r.Check(f)
	if len(got) != 1 {
		t.Fatalf("want 1 diag, got %d", len(got))
	}
	// The date value is on line 1; column starts after `@reviewed: ` (11 chars).
	if got[0].Range.Start.Line != 1 || got[0].Range.Start.Column != 12 {
		t.Errorf("expected anchor at line 1 col 12 (date value); got line %d col %d", got[0].Range.Start.Line, got[0].Range.Start.Column)
	}
}

func timeOn(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestSummarize(t *testing.T) {
	diags := []lint.Diagnostic{
		{Severity: lint.SeverityError},
		{Severity: lint.SeverityWarning},
		{Severity: lint.SeverityWarning},
	}
	s := lint.Summarize(diags)
	if s.Errors != 1 || s.Warnings != 2 {
		t.Fatalf("Summary mismatch: %+v", s)
	}
}
