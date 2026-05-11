package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePolicy_Minimal(t *testing.T) {
	src := `layer_dependencies:
  - domain MUST NOT consume infrastructure
  - application MAY consume domain

stability_tiers:
  - stable MUST NOT consume experimental
`
	p, errs := ParsePolicy("test.trellis-policy", []byte(src))
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if got, want := len(p.LayerRules), 2; got != want {
		t.Errorf("LayerRules = %d, want %d", got, want)
	}
	if got, want := len(p.StabilityRules), 1; got != want {
		t.Errorf("StabilityRules = %d, want %d", got, want)
	}
	// Provenance is recorded so diagnostics can cite the source line.
	if p.LayerRules[0].From != "test.trellis-policy" {
		t.Errorf("LayerRules[0].From = %q, want test.trellis-policy", p.LayerRules[0].From)
	}
	if p.LayerRules[0].Line != 2 {
		t.Errorf("LayerRules[0].Line = %d, want 2", p.LayerRules[0].Line)
	}
	if p.LayerRules[0].Verb != VerbMustNot {
		t.Errorf("LayerRules[0].Verb = %v, want VerbMustNot", p.LayerRules[0].Verb)
	}
	if p.LayerRules[1].Verb != VerbMay {
		t.Errorf("LayerRules[1].Verb = %v, want VerbMay", p.LayerRules[1].Verb)
	}
	if p.LayerRules[0].Source != "domain" || p.LayerRules[0].Target != "infrastructure" {
		t.Errorf("LayerRules[0] = %+v, want domain→infrastructure", p.LayerRules[0])
	}
}

func TestParsePolicy_CommentsAndBlanks(t *testing.T) {
	src := `# A header comment

# Section comment
layer_dependencies:
  # entry comment
  - domain MUST NOT consume infrastructure

`
	p, errs := ParsePolicy("", []byte(src))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(p.LayerRules) != 1 {
		t.Errorf("LayerRules = %d, want 1", len(p.LayerRules))
	}
}

func TestParsePolicy_UnknownSectionAborts(t *testing.T) {
	// Unknown section names are fatal — silent acceptance would let
	// `layer_depndencies:` (typo) become "no rules in the typo-named
	// section." Authors should hear about that immediately.
	src := `layer_typo:
  - domain MUST NOT consume infrastructure

stability_tiers:
  - stable MUST NOT consume experimental
`
	p, errs := ParsePolicy("", []byte(src))
	if len(errs) == 0 {
		t.Fatal("expected an error for unknown section, got none")
	}
	if !strings.Contains(errs[0].Message, "unknown section") {
		t.Errorf("error message = %q, want 'unknown section' substring", errs[0].Message)
	}
	// The parser stops at the unknown section: nothing past it is parsed.
	if len(p.StabilityRules) != 0 {
		t.Errorf("StabilityRules = %d after fatal error, want 0", len(p.StabilityRules))
	}
}

func TestParsePolicy_BadPredicateContinues(t *testing.T) {
	// A malformed predicate produces an error but the parser keeps going,
	// so authors see every problem at once.
	src := `layer_dependencies:
  - domain not allowed near infrastructure
  - application MAY consume domain
`
	p, errs := ParsePolicy("", []byte(src))
	if len(errs) != 1 {
		t.Fatalf("want exactly 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "invalid predicate") {
		t.Errorf("error message = %q, want 'invalid predicate' substring", errs[0].Message)
	}
	// The valid second rule still parsed.
	if len(p.LayerRules) != 1 {
		t.Errorf("LayerRules = %d, want 1 (the valid rule)", len(p.LayerRules))
	}
}

func TestParsePolicy_RuleBeforeSection(t *testing.T) {
	src := "  - domain MUST NOT consume infrastructure\n"
	_, errs := ParsePolicy("", []byte(src))
	if len(errs) != 1 {
		t.Fatalf("want 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Message, "before any section header") {
		t.Errorf("message = %q, want 'before any section header'", errs[0].Message)
	}
}

func TestMergePolicies_Concatenates(t *testing.T) {
	a := &Policy{LayerRules: []PolicyRule{{Source: "x", Verb: VerbMustNot, Target: "y"}}}
	b := &Policy{LayerRules: []PolicyRule{{Source: "y", Verb: VerbMustNot, Target: "z"}}}
	merged := MergePolicies([]*Policy{a, nil, b})
	if got := len(merged.LayerRules); got != 2 {
		t.Errorf("merged LayerRules = %d, want 2", got)
	}
}

// ---------------------------------------------------------------------
// Discovery
// ---------------------------------------------------------------------

func TestDiscoverPolicyFiles_SkipsExamples(t *testing.T) {
	// examples/ directories are intentionally skipped — example packs
	// are reference material, not active rules.
	root := t.TempDir()
	mkfile := func(rel string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("layer_dependencies:\n  - x MUST NOT consume y\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkfile("policies/architecture.trellis-policy")
	mkfile("policies/examples/sample.trellis-policy")
	mkfile("nested/team.trellis-policy")
	mkfile(".hidden/secret.trellis-policy")

	got, err := DiscoverPolicyFiles([]string{root})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	gotRel := make([]string, len(got))
	for i, p := range got {
		rel, _ := filepath.Rel(root, p)
		gotRel[i] = filepath.ToSlash(rel)
	}
	want := map[string]bool{
		"nested/team.trellis-policy":         true,
		"policies/architecture.trellis-policy": true,
	}
	for _, p := range gotRel {
		if !want[p] {
			t.Errorf("unexpected file in discovery: %s", p)
		}
		delete(want, p)
	}
	for missing := range want {
		t.Errorf("missing expected file: %s", missing)
	}
}

func TestLoadPolicyFile_ReadError(t *testing.T) {
	_, errs := LoadPolicyFile("/no/such/file.trellis-policy")
	if len(errs) != 1 {
		t.Fatalf("want 1 error, got %d", len(errs))
	}
	if errs[0].Line != 0 {
		t.Errorf("read error should have Line=0, got %d", errs[0].Line)
	}
}
