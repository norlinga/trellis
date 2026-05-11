package lint

import (
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// extractFrontmatter walks every `frontmatter_entry` in the file and
// returns a map from key (with the leading `@`) to its raw value text.
//
// "Raw value text" means whatever appears on the right of the colon,
// trimmed of surrounding whitespace. Quoted strings retain their quotes;
// callers that want the inner text should call unquote (decision #5: no
// escapes, so substring-stripping the bookend `"` is sufficient).
//
// Unlike extractOverrides this is unfiltered: every frontmatter key the
// grammar accepts ends up in the map, including `@allow-*` keys that are
// also exposed via the typed Overrides field. Callers that want
// overrides specifically should keep using HasOverride for the intent
// signal; this map is for rules that need the raw frontmatter values
// (policy enforcement reading `@layer:` and `@stability:`, drift rules,
// future ownership-aware diagnostics).
//
// Duplicate keys are tolerated — the last occurrence wins. The grammar
// does not forbid duplicates (decision #5), and a future lint rule
// (`duplicate-frontmatter-key`) is the right place to flag them.
func extractFrontmatter(tree *sitter.Tree, src []byte) map[string]string {
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
		valNode := entry.ChildByFieldName("value")
		if valNode == nil {
			continue
		}
		out[nodeText(keyNode, src)] = nodeText(valNode, src)
	}
	return out
}

// FrontmatterValue returns the raw value for key, with surrounding double
// quotes stripped for quoted-string values. Empty string when the key is
// absent.
//
// "Raw" here strips quotes but does not interpret list values (`[a, b]`
// stays as `[a, b]`). Callers that need to interpret list-shaped values
// — `@composition: [Trait1, Trait2]` — should parse the result themselves.
func (f *File) FrontmatterValue(key string) string {
	if f == nil || f.Frontmatter == nil {
		return ""
	}
	v := f.Frontmatter[key]
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

