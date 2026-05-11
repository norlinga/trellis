package lsp

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	protocol "github.com/tliron/glsp/protocol_3_16"
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/norlinga/trellis/internal/graph"
	"github.com/norlinga/trellis/internal/parser"
)

// SiteKind discriminates the two ways a handle can appear in a sidecar.
type SiteKind int

const (
	SiteProvides SiteKind = iota
	SiteConsumes
)

// HandleSite is one occurrence of a Handle inside a sidecar's AST. Ranges
// are 0-indexed (LSP convention) so they can be returned directly without
// further conversion.
//
// EntryRange covers the whole `handle_entry` (the bullet line) and is the
// jump-to-definition target. HandleRange covers just the handle text and
// anchors the hover popup so the editor highlights the right span.
type HandleSite struct {
	Path        string
	Handle      graph.Handle
	Kind        SiteKind
	EntryRange  protocol.Range
	HandleRange protocol.Range
}

// workspace is the LSP's view of every .trellis file under a single root.
//
// Built at initialize, refreshed on save. Cross-file resolution (hover,
// definition) reads from this snapshot, but the cursor's own file is always
// re-parsed from the editor's in-memory text — so unsaved edits in the active
// document don't desync the position lookup.
type workspace struct {
	mu        sync.RWMutex
	rootPath  string                        // absolute root; "" when no folder was supplied
	sites     map[string][]HandleSite       // path → all sites in that file
	sidecars  map[string]*graph.Sidecar     // path → extracted sidecar (FeatureName, FeatureSummary)
	providers map[graph.Handle][]HandleSite // handle → every Provides site of it
}

func newWorkspace() *workspace {
	return &workspace{
		sites:     map[string][]HandleSite{},
		sidecars:  map[string]*graph.Sidecar{},
		providers: map[graph.Handle][]HandleSite{},
	}
}

// load discovers and indexes every .trellis file under rootPath. Safe to
// call with an empty rootPath (e.g. when the editor opened a single file
// without a workspace folder) — the workspace is left empty and queries
// against it return nothing.
func (w *workspace) load(rootPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.rootPath = rootPath
	w.sites = map[string][]HandleSite{}
	w.sidecars = map[string]*graph.Sidecar{}
	w.providers = map[graph.Handle][]HandleSite{}
	if rootPath == "" {
		return nil
	}
	paths, err := graph.DiscoverTrellisFiles([]string{rootPath})
	if err != nil {
		return err
	}
	for _, p := range paths {
		// Per-file errors are tolerated: a broken file just doesn't contribute
		// to the index. Diagnostics for it surface through the lint loop.
		_ = w.indexFileLocked(p)
	}
	return nil
}

// reloadFile re-indexes a single file in place. Use after a save event.
func (w *workspace) reloadFile(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.dropFileLocked(path)
	_ = w.indexFileLocked(path)
}

func (w *workspace) dropFileLocked(path string) {
	delete(w.sites, path)
	delete(w.sidecars, path)
	for h, list := range w.providers {
		kept := list[:0]
		for _, s := range list {
			if s.Path != path {
				kept = append(kept, s)
			}
		}
		if len(kept) == 0 {
			delete(w.providers, h)
		} else {
			w.providers[h] = kept
		}
	}
}

func (w *workspace) indexFileLocked(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	tree, err := parser.Parse(src)
	if err != nil {
		return err
	}
	defer tree.Close()
	if tree.RootNode().HasError() {
		return errors.New("parse error")
	}
	sc, err := graph.Extract(tree, src, path)
	if err != nil {
		return err
	}
	sites := walkHandleSites(tree.RootNode(), src, path)
	w.sites[path] = sites
	w.sidecars[path] = sc
	for _, s := range sites {
		if s.Kind == SiteProvides {
			w.providers[s.Handle] = append(w.providers[s.Handle], s)
		}
	}
	return nil
}

// providersOf returns a fresh slice of every Provides site for h. Empty
// when no sidecar in this workspace provides h.
func (w *workspace) providersOf(h graph.Handle) []HandleSite {
	w.mu.RLock()
	defer w.mu.RUnlock()
	list := w.providers[h]
	if len(list) == 0 {
		return nil
	}
	out := make([]HandleSite, len(list))
	copy(out, list)
	return out
}

// sidecar returns the extracted sidecar metadata for path, or nil if the
// file isn't indexed.
func (w *workspace) sidecar(path string) *graph.Sidecar {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sidecars[path]
}

// ---------------------------------------------------------------------
// AST → HandleSite extraction
// ---------------------------------------------------------------------

// walkHandleSites returns every Provides/Consumes site in a parsed file.
// Mirrors the iteration the linter does for handle-location indexing, but
// emits LSP-native ranges so the result can be served directly.
func walkHandleSites(root *sitter.Node, src []byte, path string) []HandleSite {
	feature := root.ChildByFieldName("feature")
	if feature == nil {
		return nil
	}
	var out []HandleSite
	for i := uint(0); i < feature.NamedChildCount(); i++ {
		block := feature.NamedChild(i)
		var kind SiteKind
		switch block.Kind() {
		case "provides_block":
			kind = SiteProvides
		case "consumes_block":
			kind = SiteConsumes
		default:
			continue
		}
		for j := uint(0); j < block.NamedChildCount(); j++ {
			entry := block.NamedChild(j)
			if entry.Kind() != "handle_entry" {
				continue
			}
			handleNode := entry.ChildByFieldName("handle")
			if handleNode == nil {
				continue
			}
			h, ok := graph.ExtractHandle(handleNode, src)
			if !ok {
				continue
			}
			out = append(out, HandleSite{
				Path:        path,
				Handle:      h,
				Kind:        kind,
				EntryRange:  lspRangeFromNode(entry),
				HandleRange: lspRangeFromNode(handleNode),
			})
		}
	}
	return out
}

// resolveHandleAt parses src and returns the HandleSite under pos, if any.
// Returns nil when the cursor is not inside a handle (whitespace, prose
// inside a Scenario step, an Invariants line, etc.).
//
// pos is in LSP coordinates (0-indexed). Source comes from the editor's
// in-memory buffer, not disk, so unsaved edits are reflected.
func resolveHandleAt(path string, src []byte, pos protocol.Position) *HandleSite {
	tree, err := parser.Parse(src)
	if err != nil {
		return nil
	}
	defer tree.Close()
	root := tree.RootNode()
	point := sitter.Point{Row: uint(pos.Line), Column: uint(pos.Character)}
	desc := root.NamedDescendantForPointRange(point, point)
	if desc == nil {
		return nil
	}

	handleNode := ascendTo(desc, "path_handle", "prefixed_handle")
	if handleNode == nil {
		return nil
	}
	h, ok := graph.ExtractHandle(handleNode, src)
	if !ok {
		return nil
	}
	entry := ascendTo(handleNode, "handle_entry")
	if entry == nil {
		return nil
	}
	block := ascendTo(entry, "provides_block", "consumes_block")
	if block == nil {
		return nil
	}
	var kind SiteKind
	switch block.Kind() {
	case "provides_block":
		kind = SiteProvides
	case "consumes_block":
		kind = SiteConsumes
	}
	return &HandleSite{
		Path:        path,
		Handle:      h,
		Kind:        kind,
		EntryRange:  lspRangeFromNode(entry),
		HandleRange: lspRangeFromNode(handleNode),
	}
}

// ascendTo walks up from n looking for the nearest ancestor whose Kind matches
// any of kinds. Returns nil if no match is found before the root.
func ascendTo(n *sitter.Node, kinds ...string) *sitter.Node {
	for cur := n; cur != nil; cur = cur.Parent() {
		k := cur.Kind()
		for _, want := range kinds {
			if k == want {
				return cur
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------
// LSP ↔ tree-sitter coordinate conversion
// ---------------------------------------------------------------------

// lspRangeFromNode produces a 0-indexed protocol.Range from a tree-sitter
// node. Both systems are 0-indexed for line and column, so this is a
// straight type cast.
func lspRangeFromNode(n *sitter.Node) protocol.Range {
	s, e := n.StartPosition(), n.EndPosition()
	return protocol.Range{
		Start: protocol.Position{Line: uint32(s.Row), Character: uint32(s.Column)},
		End:   protocol.Position{Line: uint32(e.Row), Character: uint32(e.Column)},
	}
}

// pathToURI converts an absolute filesystem path to a `file://` URI. Used
// when constructing definition responses so the editor can navigate.
func pathToURI(path string) string {
	if strings.HasPrefix(path, "file://") {
		return path
	}
	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	return u.String()
}

// rootPathFromInitialize picks the workspace root from the InitializeParams,
// preferring (in order): the first WorkspaceFolder, RootURI, RootPath.
// Returns "" if the client didn't supply any of them.
func rootPathFromInitialize(params *protocol.InitializeParams) string {
	if len(params.WorkspaceFolders) > 0 {
		return uriToPath(string(params.WorkspaceFolders[0].URI))
	}
	if params.RootURI != nil && *params.RootURI != "" {
		return uriToPath(string(*params.RootURI))
	}
	if params.RootPath != nil && *params.RootPath != "" {
		return *params.RootPath
	}
	return ""
}
