package lint

import (
	"fmt"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// requiredFrontmatterKeys are the four core keys the linter expects in
// every sidecar. The decision-doc set (#5a) is five keys including
// @composition; @composition is treated as recommended-not-required because
// many units do not declare traits.
//
// `spec/format.md` Authoring Conventions §Frontmatter codifies this stance.
var requiredFrontmatterKeys = []string{"@owner", "@stability", "@since", "@reviewed"}

// FrontmatterRequiredKeys warns once per missing required frontmatter key.
type FrontmatterRequiredKeys struct{}

func (*FrontmatterRequiredKeys) Code() string { return "frontmatter-missing-required" }

func (*FrontmatterRequiredKeys) Check(f *File) []Diagnostic {
	root := f.Tree.RootNode()
	seen := make(map[string]bool, 4)
	if fm := root.ChildByFieldName("frontmatter"); fm != nil {
		for i := uint(0); i < fm.NamedChildCount(); i++ {
			entry := fm.NamedChild(i)
			if entry.Kind() != "frontmatter_entry" {
				continue
			}
			if keyNode := entry.ChildByFieldName("key"); keyNode != nil {
				seen[nodeText(keyNode, f.Source)] = true
			}
		}
	}

	// Anchor missing-key diagnostics to the Feature line if present;
	// otherwise to the file start. The Feature line is more useful in an
	// editor — clicking it scrolls to where you'd add the frontmatter
	// (immediately above) — but a missing-feature file would also be
	// missing all four required keys, so falling back to file start is
	// still informative.
	anchor := FileStart()
	if feat := root.ChildByFieldName("feature"); feat != nil {
		anchor = RangeFromNode(feat)
	}

	var out []Diagnostic
	for _, key := range requiredFrontmatterKeys {
		if !seen[key] {
			out = append(out, Diagnostic{
				Code:     "frontmatter-missing-required",
				Severity: SeverityWarning,
				Message:  "missing required frontmatter key: " + key,
				Path:     f.Path,
				Range:    anchor,
			})
		}
	}
	return out
}

// canonicalScenarioKinds is the v1 enumerated set from decision #8.
// Linter-only — the grammar accepts any kebab-case identifier.
var canonicalScenarioKinds = map[string]bool{
	"happy-path":  true,
	"negative":    true,
	"edge":        true,
	"regression":  true,
	"security":    true,
	"performance": true,
}

// scenarioKindAliases maps known aliases to their canonical form (decision
// #8). Aliases drive diagnostic suggestions only — files MUST use the
// canonical form. Adding to this table is safe; it does not expand the
// canonical set or risk file-format breakage.
var scenarioKindAliases = map[string]string{
	"success":         "happy-path",
	"happy":           "happy-path",
	"nominal":         "happy-path",
	"correct":         "happy-path",
	"golden-path":     "happy-path",
	"failure":         "negative",
	"fail":            "negative",
	"error":           "negative",
	"unhappy":         "negative",
	"boundary":        "edge",
	"corner":          "edge",
	"race":            "edge",
	"concurrency":     "edge",
	"concurrent":      "edge",
	"regression-test": "regression",
	"bugfix":          "regression",
	"bug-fix":         "regression",
	"auth":            "security",
	"authn":           "security",
	"authz":           "security",
	"perf":            "performance",
	"load":            "performance",
	"latency":         "performance",
}

// ScenarioKindCanonical warns on any kind that is not in the canonical
// set. Two messages: one for known aliases (suggests the canonical), one
// for unknown identifiers (no suggestion in v1; edit-distance hints are a
// future enhancement).
type ScenarioKindCanonical struct{}

func (*ScenarioKindCanonical) Code() string { return "scenario-kind-canonical" }

func (*ScenarioKindCanonical) Check(f *File) []Diagnostic {
	feature := f.Tree.RootNode().ChildByFieldName("feature")
	if feature == nil {
		return nil
	}
	var out []Diagnostic
	for i := uint(0); i < feature.NamedChildCount(); i++ {
		sb := feature.NamedChild(i)
		if sb.Kind() != "scenario_block" {
			continue
		}
		kindNode := sb.ChildByFieldName("kind")
		if kindNode == nil {
			continue // missing-kind is a different (future) rule
		}
		kind := nodeText(kindNode, f.Source)
		if canonicalScenarioKinds[kind] {
			continue
		}
		var msg string
		if cano, ok := scenarioKindAliases[kind]; ok {
			msg = fmt.Sprintf("'%s' is a recognized alias; the canonical form in Trellis is '%s'", kind, cano)
		} else {
			msg = fmt.Sprintf("'%s' is not a canonical scenario kind", kind)
		}
		out = append(out, Diagnostic{
			Code:     "scenario-kind-canonical",
			Severity: SeverityWarning,
			Message:  msg,
			Path:     f.Path,
			Range:    RangeFromNode(kindNode),
		})
	}
	return out
}

func nodeText(n *sitter.Node, src []byte) string {
	return string(src[n.StartByte():n.EndByte()])
}
