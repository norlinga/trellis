package lint

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// SourceAnchorShape validates the basic @source("kind:target") payload. The
// grammar keeps the payload opaque so language ecosystems can define useful
// anchors without changing handle identity; this rule catches malformed
// anchors early while allowing new kinds.
type SourceAnchorShape struct{}

func (*SourceAnchorShape) Code() string { return "source-anchor-shape" }

func (*SourceAnchorShape) Check(f *File) []Diagnostic {
	feature := f.Tree.RootNode().ChildByFieldName("feature")
	if feature == nil {
		return nil
	}
	var out []Diagnostic
	for i := uint(0); i < feature.NamedChildCount(); i++ {
		block := feature.NamedChild(i)
		if block.Kind() != "provides_block" && block.Kind() != "consumes_block" {
			continue
		}
		for j := uint(0); j < block.NamedChildCount(); j++ {
			entry := block.NamedChild(j)
			if entry.Kind() != "handle_entry" {
				continue
			}
			anchor := entry.ChildByFieldName("source_anchor")
			if anchor == nil {
				continue
			}
			valueNode := anchor.ChildByFieldName("value")
			if valueNode == nil {
				continue
			}
			value := unquoteAnchorValue(nodeText(valueNode, f.Source))
			if msg := validateSourceAnchorValue(value); msg != "" {
				out = append(out, Diagnostic{
					Code:     "source-anchor-shape",
					Severity: SeverityWarning,
					Message:  msg,
					Path:     f.Path,
					Range:    RangeFromNode(anchor),
				})
			}
		}
	}
	return out
}

func unquoteAnchorValue(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func validateSourceAnchorValue(value string) string {
	kind, target, ok := strings.Cut(value, ":")
	if !ok {
		return fmt.Sprintf("source anchor %q must use kind:target form, such as label:PARAGRAPH or line:42-68", value)
	}
	if !validAnchorKind(kind) {
		return fmt.Sprintf("source anchor kind %q must start with a letter and contain only letters, digits, '_' or '-'", kind)
	}
	if strings.TrimSpace(target) == "" {
		return fmt.Sprintf("source anchor %q must include a non-empty target", value)
	}
	if kind == "line" {
		return validateLineAnchor(target)
	}
	return ""
}

func validAnchorKind(kind string) bool {
	if kind == "" {
		return false
	}
	for i, r := range kind {
		if i == 0 {
			if !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func validateLineAnchor(target string) string {
	start, end, ok := strings.Cut(target, "-")
	if !ok {
		n, err := strconv.Atoi(target)
		if err != nil || n < 1 {
			return fmt.Sprintf("line source anchor %q must be a positive line number or range", target)
		}
		return ""
	}
	s, errS := strconv.Atoi(start)
	e, errE := strconv.Atoi(end)
	if errS != nil || errE != nil || s < 1 || e < 1 {
		return fmt.Sprintf("line source anchor %q must be a positive line range", target)
	}
	if e < s {
		return fmt.Sprintf("line source anchor %q must not end before it starts", target)
	}
	return ""
}
