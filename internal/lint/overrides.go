package lint

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// OverridePrefix is the frontmatter key prefix that marks a lint
// override. Whitepaper §4.2 names overrides like `@allow-many-scenarios`;
// every override key the linter recognizes starts with `@allow-`.
const OverridePrefix = "@allow-"

// extractOverrides walks the file's frontmatter and returns a map from
// override key (with the leading `@`) to the justification text. Only
// quoted-string values are accepted — the friction principle requires a
// human-readable reason for each suppression.
//
// Non-quoted values (identifiers, dates, lists) at an `@allow-*` key are
// silently ignored: the grammar accepts them but they don't carry the
// documentation artifact the override mechanism is meant to produce.
// Future work: emit a diagnostic on `@allow-X: <unquoted>` so authors
// don't accidentally bypass the friction.
func extractOverrides(tree *sitter.Tree, src []byte) map[string]string {
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
		key := nodeText(keyNode, src)
		if !strings.HasPrefix(key, OverridePrefix) {
			continue
		}
		valNode := entry.ChildByFieldName("value")
		if valNode == nil || valNode.Kind() != "quoted_string" {
			continue
		}
		raw := nodeText(valNode, src)
		// Strip the surrounding double quotes the grammar guarantees.
		if len(raw) >= 2 {
			raw = raw[1 : len(raw)-1]
		}
		out[key] = raw
	}
	return out
}

// HasOverride reports whether the file declares the given override key
// (e.g., "@allow-many-consumes"). The justification text is intentionally
// not returned here — rules consult the file map directly when they want
// to surface the reason in a diagnostic message.
func (f *File) HasOverride(key string) bool {
	if f.Overrides == nil {
		return false
	}
	_, ok := f.Overrides[key]
	return ok
}
