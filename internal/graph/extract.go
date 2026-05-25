package graph

import (
	"fmt"
	"strconv"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Extract walks a parsed Trellis AST and materializes a Sidecar. The path
// argument is recorded on the result; it is not used to interpret the AST.
//
// Extract assumes a clean parse — the caller must reject HasError trees
// before invoking this function. Anything Extract sees that doesn't match
// the expected shape is silently skipped, on the principle that the parser
// is the source of truth for shape and the extractor's job is just
// projection.
func Extract(tree *sitter.Tree, src []byte, path string) (*Sidecar, error) {
	root := tree.RootNode()
	if root.Kind() != "source_file" {
		return nil, fmt.Errorf("expected source_file root, got %s", root.Kind())
	}
	feature := root.ChildByFieldName("feature")
	if feature == nil {
		return nil, fmt.Errorf("no feature in %s", path)
	}

	sc := &Sidecar{
		Path:       path,
		SourcePath: SourcePathFor(path),
	}
	if name := feature.ChildByFieldName("name"); name != nil {
		sc.FeatureName = strings.TrimSpace(nodeText(name, src))
	}
	if sum := feature.ChildByFieldName("summary"); sum != nil {
		sc.FeatureSummary = unquote(nodeText(sum, src))
	}

	for i := uint(0); i < feature.NamedChildCount(); i++ {
		child := feature.NamedChild(i)
		switch child.Kind() {
		case "provides_block":
			sc.Provides = extractHandleEntries(child, src)
		case "consumes_block":
			sc.Consumes = extractHandleEntries(child, src)
		}
	}
	return sc, nil
}

func extractHandleEntries(block *sitter.Node, src []byte) []Entry {
	var out []Entry
	for i := uint(0); i < block.NamedChildCount(); i++ {
		entry := block.NamedChild(i)
		if entry.Kind() != "handle_entry" {
			continue
		}
		handleNode := entry.ChildByFieldName("handle")
		if handleNode == nil {
			continue
		}
		h, ok := extractHandle(handleNode, src)
		if !ok {
			continue
		}
		desc := ""
		if d := entry.ChildByFieldName("description"); d != nil {
			desc = strings.TrimSpace(nodeText(d, src))
		}
		var anchor *SourceAnchor
		if a := entry.ChildByFieldName("source_anchor"); a != nil {
			anchor = extractSourceAnchor(a, src)
		}
		out = append(out, Entry{Handle: h, SourceAnchor: anchor, Description: desc})
	}
	return out
}

// ExtractHandle reads a single handle node (path_handle or prefixed_handle)
// and returns the materialized Handle. Exposed for callers — like the
// linter — that need to map AST nodes to handles independently of the
// per-file Extract pass.
func ExtractHandle(node *sitter.Node, src []byte) (Handle, bool) {
	return extractHandle(node, src)
}

func extractHandle(node *sitter.Node, src []byte) (Handle, bool) {
	switch node.Kind() {
	case "path_handle":
		// path_handle wraps a single identifier_path. Reading the node's
		// byte range yields the dotted form intact — identifiers cannot
		// contain whitespace, so no intra-token cleanup is needed.
		pathNode := node.NamedChild(0)
		if pathNode == nil {
			return Handle{}, false
		}
		return Handle{Kind: PathHandle, Path: nodeText(pathNode, src)}, true
	case "prefixed_handle":
		prefix := node.ChildByFieldName("prefix")
		pathNode := node.ChildByFieldName("path")
		if prefix == nil || pathNode == nil {
			return Handle{}, false
		}
		return Handle{
			Kind:   PrefixedHandle,
			Prefix: nodeText(prefix, src),
			Path:   nodeText(pathNode, src),
		}, true
	}
	return Handle{}, false
}

func extractSourceAnchor(node *sitter.Node, src []byte) *SourceAnchor {
	valueNode := node.ChildByFieldName("value")
	if valueNode == nil {
		return nil
	}
	return ParseSourceAnchor(unquote(nodeText(valueNode, src)))
}

// ParseSourceAnchor materializes the opaque @source("kind:target") payload
// into a SourceAnchor. Unknown kinds are preserved as Kind/Target pairs; line
// anchors also expose numeric bounds for consumers that need ranges.
func ParseSourceAnchor(value string) *SourceAnchor {
	a := &SourceAnchor{Value: value}
	kind, target, ok := strings.Cut(value, ":")
	if !ok {
		return a
	}
	a.Kind = kind
	a.Target = target
	if kind == "line" {
		parseLineAnchor(a, target)
	}
	return a
}

func parseLineAnchor(a *SourceAnchor, target string) {
	start, end, ok := strings.Cut(target, "-")
	if !ok {
		if n, err := strconv.Atoi(target); err == nil {
			a.StartLine = n
			a.EndLine = n
		}
		return
	}
	s, errS := strconv.Atoi(start)
	e, errE := strconv.Atoi(end)
	if errS == nil && errE == nil {
		a.StartLine = s
		a.EndLine = e
	}
}

func nodeText(n *sitter.Node, src []byte) string {
	return string(src[n.StartByte():n.EndByte()])
}

// unquote strips the surrounding double quotes from a quoted_string node's
// byte range. Grammar guarantees the leading and trailing `"`; no escape
// processing is needed (decision #5: no escapes in v1 strings).
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
