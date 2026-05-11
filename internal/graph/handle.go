// Package graph builds and queries the Provides:/Consumes: dependency graph
// over a workspace of .trellis sidecar files.
//
// The handle/description split and graph queries encoded here are
// described in `spec/format.md` (the authoring guide) and surfaced as
// annotated comments in the tree-sitter grammar at
// `github.com/norlinga/tree-sitter-trellis`.
package graph

import "strings"

// HandleKind discriminates the two handle shapes the grammar produces
// (decision #6). The kind is preserved because, for diagnostics, an
// unresolved `Event:foo` should not be conflated with an unresolved `Event`
// path-handle that happens to share a name.
type HandleKind int

const (
	PathHandle     HandleKind = iota // leftmost dotted-identifier path, e.g. Subscription.create
	PrefixedHandle                   // prefix + : + path, e.g. Event:subscription.created
)

// Handle is the canonical name used for graph matching. Two Handles compare
// equal iff their kind, prefix, and path are bit-for-bit equal — case
// sensitive (decision #6 explicitly: `Subscription.Create` ≠
// `Subscription.create`). Description text is not part of the handle and is
// never consulted by the graph.
type Handle struct {
	Kind   HandleKind
	Prefix string // empty unless Kind == PrefixedHandle
	Path   string // dotted identifier path, e.g. "Subscription.create"
}

// String renders the handle in its canonical on-disk form. Round-trips with
// the parser's tokenization for both kinds.
func (h Handle) String() string {
	if h.Kind == PrefixedHandle {
		return h.Prefix + ":" + h.Path
	}
	return h.Path
}

// Root is the leftmost segment of the path. Useful for grouping diagnostics
// (e.g. "Subscription.create and Subscription.cancel both belong to
// Subscription"). Linter territory mostly; included here because the graph
// builder already has the path tokenized.
func (h Handle) Root() string {
	if i := strings.IndexByte(h.Path, '.'); i >= 0 {
		return h.Path[:i]
	}
	return h.Path
}
