package lint_test

import (
	"strings"
	"testing"

	"github.com/norlinga/trellis/internal/lint"
)

func TestLintSingleFile_Clean(t *testing.T) {
	src := `@owner: T
@stability: stable
@since: 2026-01-01
@reviewed: 2026-05-10

Feature: F
  "fully populated"

  Provides:
    - F.do

  Invariants:
    - it MUST stay true

  Scenario (happy-path): h
    Given x

  Scenario (negative): n
    Given y
`
	diags := lint.LintSingleFile("/x/foo.go.trellis", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("clean fixture should yield 0 diagnostics, got %v", diags)
	}
}

func TestLintSingleFile_FiresPerFileRules(t *testing.T) {
	src := `Feature: F
  "deficient — no frontmatter, no invariants, no negative scenario"

  Scenario (happy): h
    Given x
`
	diags := lint.LintSingleFile("/x/foo.go.trellis", []byte(src))
	codes := map[string]bool{}
	for _, d := range diags {
		codes[d.Code] = true
	}
	for _, want := range []string{
		"frontmatter-missing-required",
		"scenario-kind-canonical",
		"missing-invariants",
		"missing-negative-scenario",
	} {
		if !codes[want] {
			t.Errorf("expected diagnostic with code %q in %v", want, codes)
		}
	}
}

func TestLintSingleFile_DoesNotFireWorkspaceRules(t *testing.T) {
	// A sidecar consuming a non-external handle WOULD fire broken-link
	// in workspace mode. LintSingleFile only runs per-file rules, so
	// this should produce no broken-link diagnostic.
	src := `@owner: T
@stability: stable
@since: 2026-01-01
@reviewed: 2026-05-10

Feature: F
  "x"

  Provides:
    - F.do

  Consumes:
    - SomeOther.thing

  Invariants:
    - it MUST stay true

  Scenario (happy-path): h
    Given x

  Scenario (negative): n
    Given y
`
	diags := lint.LintSingleFile("/x/foo.go.trellis", []byte(src))
	for _, d := range diags {
		if d.Code == "broken-link" {
			t.Errorf("LintSingleFile should not run broken-link (workspace rule); got %v", d)
		}
	}
}

func TestLintSingleFile_ParseError(t *testing.T) {
	src := `this is not a sidecar at all
no Feature line
`
	diags := lint.LintSingleFile("/x/garbage.trellis", []byte(src))
	if len(diags) != 1 {
		t.Fatalf("expected 1 parse-error diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Severity != lint.SeverityError {
		t.Errorf("parse error should be SeverityError; got %v", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Code, "parse") {
		t.Errorf("expected code to mention 'parse'; got %q", diags[0].Code)
	}
}
