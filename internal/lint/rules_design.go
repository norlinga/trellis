package lint

import (
	"fmt"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// scenarioKinds returns the kind label for every scenario_block in the
// feature. Empty string entries represent kindless scenarios (`Scenario:`
// with no parens). Order follows AST order.
func scenarioKinds(feature *sitter.Node, src []byte) []string {
	if feature == nil {
		return nil
	}
	var out []string
	for i := uint(0); i < feature.NamedChildCount(); i++ {
		sb := feature.NamedChild(i)
		if sb.Kind() != "scenario_block" {
			continue
		}
		if k := sb.ChildByFieldName("kind"); k != nil {
			out = append(out, nodeText(k, src))
		} else {
			out = append(out, "")
		}
	}
	return out
}

// findChild returns the first named child of feature whose Kind matches
// any of the given kinds. nil if none.
func findChild(feature *sitter.Node, kinds ...string) *sitter.Node {
	if feature == nil {
		return nil
	}
	for i := uint(0); i < feature.NamedChildCount(); i++ {
		c := feature.NamedChild(i)
		for _, k := range kinds {
			if c.Kind() == k {
				return c
			}
		}
	}
	return nil
}

// ScenarioCount enforces the whitepaper §4.2 SRP-proxy threshold. The
// canonical defaults are 10 (warn) and 15 (error); callers may override
// for tests or future policy-file consumption.
type ScenarioCount struct{ Warn, Error int }

func (*ScenarioCount) Code() string { return "scenario-count" }

func (r *ScenarioCount) Check(f *File) []Diagnostic {
	if f.HasOverride("@allow-many-scenarios") {
		return nil
	}
	feature := f.Tree.RootNode().ChildByFieldName("feature")
	kinds := scenarioKinds(feature, f.Source)
	n := len(kinds)
	if n < r.Warn {
		return nil
	}
	sev := SeverityWarning
	if n >= r.Error {
		sev = SeverityError
	}
	anchor := FileStart()
	if feature != nil {
		anchor = RangeFromNode(feature)
	}
	return []Diagnostic{{
		Code:     "scenario-count",
		Severity: sev,
		Message:  fmt.Sprintf("feature has %d scenarios (threshold: warn at %d, error at %d) — consider splitting the unit", n, r.Warn, r.Error),
		Path:     f.Path,
		Range:    anchor,
	}}
}

// ConsumesCount enforces the whitepaper §4.2 coupling-proxy threshold.
type ConsumesCount struct{ Warn, Error int }

func (*ConsumesCount) Code() string { return "consumes-count" }

func (r *ConsumesCount) Check(f *File) []Diagnostic {
	if f.HasOverride("@allow-many-consumes") {
		return nil
	}
	n := len(f.Sidecar.Consumes)
	if n < r.Warn {
		return nil
	}
	sev := SeverityWarning
	if n >= r.Error {
		sev = SeverityError
	}
	feature := f.Tree.RootNode().ChildByFieldName("feature")
	anchor := FileStart()
	if cb := findChild(feature, "consumes_block"); cb != nil {
		anchor = RangeFromNode(cb)
	} else if feature != nil {
		anchor = RangeFromNode(feature)
	}
	return []Diagnostic{{
		Code:     "consumes-count",
		Severity: sev,
		Message:  fmt.Sprintf("feature consumes %d handles (threshold: warn at %d, error at %d) — high coupling indicates a coordinator that should be split", n, r.Warn, r.Error),
		Path:     f.Path,
		Range:    anchor,
	}}
}

// MissingNegativeScenario warns when a feature has at least one scenario
// but no scenario tagged with the canonical 'negative' kind. Aliases are
// not counted — ScenarioKindCanonical handles those — so an author who
// wrote `Scenario (failure):` will see two warnings (canonicalize, AND
// missing negative). That's intentional: silencing the first by
// canonicalizing also silences the second.
type MissingNegativeScenario struct{}

func (*MissingNegativeScenario) Code() string { return "missing-negative-scenario" }

func (*MissingNegativeScenario) Check(f *File) []Diagnostic {
	if f.HasOverride("@allow-no-negative-scenario") {
		return nil
	}
	feature := f.Tree.RootNode().ChildByFieldName("feature")
	kinds := scenarioKinds(feature, f.Source)
	if len(kinds) == 0 {
		// A feature with no scenarios is its own (different) signal —
		// don't double-fire here.
		return nil
	}
	for _, k := range kinds {
		if k == "negative" {
			return nil
		}
	}
	anchor := FileStart()
	if feature != nil {
		anchor = RangeFromNode(feature)
	}
	return []Diagnostic{{
		Code:     "missing-negative-scenario",
		Severity: SeverityWarning,
		Message:  "feature has no 'negative' scenario — almost every real unit has at least one failure mode worth pinning",
		Path:     f.Path,
		Range:    anchor,
	}}
}

// MissingInvariants warns when a feature has no Invariants: block at all.
// Whitepaper §4.2: an empty/missing invariants block "is often a sign the
// spec is just transcribing the code rather than abstracting over it."
//
// The override mechanism (slice F) will let authors silence this with a
// `@allow-no-invariants:` frontmatter key when the unit genuinely has no
// invariants worth stating.
type MissingInvariants struct{}

func (*MissingInvariants) Code() string { return "missing-invariants" }

func (*MissingInvariants) Check(f *File) []Diagnostic {
	if f.HasOverride("@allow-no-invariants") {
		return nil
	}
	feature := f.Tree.RootNode().ChildByFieldName("feature")
	if feature == nil {
		return nil
	}
	if findChild(feature, "invariants_block") != nil {
		return nil
	}
	return []Diagnostic{{
		Code:     "missing-invariants",
		Severity: SeverityWarning,
		Message:  "feature has no Invariants: block — declare what MUST be true about this unit, or omit only after deliberate consideration",
		Path:     f.Path,
		Range:    RangeFromNode(feature),
	}}
}
