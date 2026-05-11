package lint_test

import (
	"strings"
	"testing"

	"github.com/norlinga/trellis/internal/graph"
	"github.com/norlinga/trellis/internal/lint"
)

// Each rule that supports an override must (a) silence its diagnostic
// when the override is present, (b) still fire when the override is
// absent. These tests are intentionally thin — they don't re-test the
// rule's normal behavior, just the suppression hook.

func TestOverride_ScenarioCount(t *testing.T) {
	withOverride := parseFixture(t, `@allow-many-scenarios: "state machine with N legitimate states"

`+makeScenarios(12))
	withoutOverride := parseFixture(t, makeScenarios(12))

	if got := (&lint.ScenarioCount{Warn: 10, Error: 15}).Check(withOverride); len(got) != 0 {
		t.Errorf("override should silence; got %v", got)
	}
	if got := (&lint.ScenarioCount{Warn: 10, Error: 15}).Check(withoutOverride); len(got) != 1 {
		t.Errorf("no override should fire; got %v", got)
	}
}

func TestOverride_ConsumesCount(t *testing.T) {
	withOverride := parseFixture(t, `@allow-many-consumes: "wrapper file"

`+makeConsumes(9))
	withoutOverride := parseFixture(t, makeConsumes(9))

	if got := (&lint.ConsumesCount{Warn: 5, Error: 8}).Check(withOverride); len(got) != 0 {
		t.Errorf("override should silence; got %v", got)
	}
	if got := (&lint.ConsumesCount{Warn: 5, Error: 8}).Check(withoutOverride); len(got) != 1 {
		t.Errorf("no override should fire; got %v", got)
	}
}

func TestOverride_MissingInvariants(t *testing.T) {
	withOverride := parseFixture(t, `@allow-no-invariants: "trivial value type"

Feature: F
  "x"
`)
	withoutOverride := parseFixture(t, `Feature: F
  "x"
`)
	if got := (&lint.MissingInvariants{}).Check(withOverride); len(got) != 0 {
		t.Errorf("override should silence; got %v", got)
	}
	if got := (&lint.MissingInvariants{}).Check(withoutOverride); len(got) != 1 {
		t.Errorf("no override should fire; got %v", got)
	}
}

func TestOverride_MissingNegativeScenario(t *testing.T) {
	withOverride := parseFixture(t, `@allow-no-negative-scenario: "navigation file"

Feature: F
  "x"

  Scenario (happy-path): h
    Given x
`)
	if got := (&lint.MissingNegativeScenario{}).Check(withOverride); len(got) != 0 {
		t.Errorf("override should silence; got %v", got)
	}
}

// extractOverrides skips non-quoted-string values, so an author who
// writes `@allow-many-consumes: bareword` does NOT get the suppression.
// The friction principle is the whole point — bypassing it must be hard.
func TestOverride_NonQuotedValueIgnored(t *testing.T) {
	f := parseFixture(t, `@allow-many-consumes: bareword

`+makeConsumes(9))
	if !f.HasOverride("@allow-many-consumes") {
		// Identifier values aren't picked up — that's the contract.
		// (Test asserts the negative.)
	} else {
		t.Errorf("non-quoted value should not register as an override")
	}
	if got := (&lint.ConsumesCount{Warn: 5, Error: 8}).Check(f); len(got) != 1 {
		t.Errorf("rule should still fire when override value is non-quoted; got %d diags", len(got))
	}
}

func TestOverride_PreservesJustification(t *testing.T) {
	f := parseFixture(t, `@allow-many-consumes: "wrapper file; collapses external API surface"

Feature: F
  "x"

  Provides:
    - F.do
`)
	got := f.Overrides["@allow-many-consumes"]
	if !strings.Contains(got, "wrapper file") {
		t.Errorf("justification text not preserved verbatim; got %q", got)
	}
}

// Test through the full Default linter — make sure the override hooks
// still work when rules are exercised via the real Default() rule set.
func TestOverride_DefaultLinterRespectsAll(t *testing.T) {
	f := parseFixture(t, `@owner: T
@stability: stable
@since: 2026-01-01
@reviewed: 2026-05-10
@allow-many-consumes: "wrapper"
@allow-no-invariants: "trivial"
@allow-no-negative-scenario: "navigation"
@allow-many-scenarios: "state machine"

Feature: F
  "x"

`+strings.Repeat("  Consumes:\n    - h.a\n", 1)+`
  Scenario (happy-path): h
    Given x
`)
	g := graph.Build([]*graph.Sidecar{f.Sidecar})
	diags := lint.Default().Lint([]*lint.File{f}, g)
	for _, d := range diags {
		switch d.Code {
		case "consumes-count", "missing-invariants", "missing-negative-scenario", "scenario-count":
			t.Errorf("rule %s should be silenced by override; got %v", d.Code, d)
		}
	}
}
