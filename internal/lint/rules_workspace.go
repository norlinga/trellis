package lint

import (
	"fmt"
	"os"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/norlinga/trellis/internal/graph"
)

// DefaultExternalPrefixes is the v1 hardcoded allowlist of first-segment
// names that BrokenLink treats as external — i.e., a Consumes handle
// whose Root() matches one of these is a known import, not a broken link.
//
// Long-term, this list moves into the policy file (deliverable #6) so
// each org can declare its own external surface. Until then the default
// covers Go stdlib and the toolchain's own dependencies; users with other
// stacks can construct a custom BrokenLink with a different set.
var DefaultExternalPrefixes = []string{
	// Go stdlib top-level packages a sidecar might reasonably reference.
	"os", "filepath", "fs", "io", "fmt", "errors", "strings", "bytes",
	"sort", "path", "time", "context", "sync", "regexp", "unicode",
	"runtime", "reflect", "encoding", "math", "log", "flag",
	"slices", "maps", "iter",

	// CLI / config ecosystem.
	"cobra", "viper", "pflag",

	// Trellis-side dependencies.
	"sitter", "tree_sitter_trellis",
}

// BrokenLink reports Consumes handles that resolved to no provider in the
// workspace. Path handles whose Root() is in ExternalPrefixes are silently
// skipped; prefixed handles (Event:, Trait:) are never filtered because
// those domains are author-controlled and a broken Event: handle is more
// likely a real bug.
type BrokenLink struct {
	ExternalPrefixes map[string]bool
}

// NewBrokenLink returns a BrokenLink seeded with DefaultExternalPrefixes.
func NewBrokenLink() *BrokenLink {
	set := make(map[string]bool, len(DefaultExternalPrefixes))
	for _, p := range DefaultExternalPrefixes {
		set[p] = true
	}
	return &BrokenLink{ExternalPrefixes: set}
}

func (*BrokenLink) Code() string { return "broken-link" }

func (r *BrokenLink) Check(files []*File, g *graph.Graph) []Diagnostic {
	if g == nil {
		return nil
	}
	locs := buildHandleLocations(files)
	var out []Diagnostic
	for _, u := range g.Unresolved {
		if u.Handle.Kind == graph.PathHandle && r.ExternalPrefixes[u.Handle.Root()] {
			continue
		}
		rng := FileStart()
		if r, ok := locs[handleSiteKey{Path: u.From.Path, Handle: u.Handle}]; ok {
			rng = r
		}
		out = append(out, Diagnostic{
			Code:     "broken-link",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Consumes: %s has no provider in this workspace", u.Handle),
			Path:     u.From.Path,
			Range:    rng,
		})
	}
	return out
}

// DuplicateProvides errors once per provider site when a handle is
// declared by more than one sidecar. Whitepaper §4.4 calls this out as
// the "duplicate features at write time" check; decision #6 confirms
// handle-equality is the right matching primitive (fuzzy is deferred).
type DuplicateProvides struct{}

func (*DuplicateProvides) Code() string { return "duplicate-provides" }

func (*DuplicateProvides) Check(files []*File, g *graph.Graph) []Diagnostic {
	if g == nil {
		return nil
	}
	locs := buildHandleLocations(files)
	var out []Diagnostic
	for h, providers := range g.DuplicateProvides() {
		others := len(providers) - 1
		for _, sc := range providers {
			rng := FileStart()
			if r, ok := locs[handleSiteKey{Path: sc.Path, Handle: h}]; ok {
				rng = r
			}
			out = append(out, Diagnostic{
				Code:     "duplicate-provides",
				Severity: SeverityError,
				Message:  fmt.Sprintf("handle %s is also provided by %d other sidecar(s); each handle MUST have exactly one provider", h, others),
				Path:     sc.Path,
				Range:    rng,
			})
		}
	}
	return out
}

// OrphanSourceFile warns when a sidecar's paired source file does not
// exist on disk. The pairing convention is `<source>.<ext>.trellis` — the
// linter strips `.trellis` and Stats the result.
//
// This is the "source-file-missing orphan" half of whitepaper §4.1's
// orphan-sidecars check (the other half — Provides with no consumers — is
// graph.Orphans()). The linter exposes both as separate signals because
// they suggest different remediations: missing source = stale sidecar to
// delete; no consumers = unused unit to question.
type OrphanSourceFile struct{}

func (*OrphanSourceFile) Code() string { return "orphan-source-file" }

func (*OrphanSourceFile) Check(files []*File, g *graph.Graph) []Diagnostic {
	var out []Diagnostic
	for _, f := range files {
		if f.HasOverride("@allow-orphan-source") {
			continue
		}
		sp := f.Sidecar.SourcePath
		if sp == "" {
			continue // not a *.<ext>.trellis path; format violation handled elsewhere
		}
		if _, err := os.Stat(sp); err == nil {
			continue
		}
		out = append(out, Diagnostic{
			Code:     "orphan-source-file",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("paired source file does not exist: %s — the sidecar may be stale", sp),
			Path:     f.Path,
			Range:    FileStart(),
		})
	}
	return out
}

// ---------------------------------------------------------------------
// Handle-location index used by workspace rules to anchor diagnostics
// at the specific handle_entry node instead of the file start.
// ---------------------------------------------------------------------

type handleSiteKey struct {
	Path   string
	Handle graph.Handle
}

// buildHandleLocations walks every file's AST and builds an index from
// (file path, handle) → AST range of the handle_entry node. The result
// is consulted by BrokenLink and DuplicateProvides to make their
// diagnostics IDE-jumpable.
//
// Iteration is in AST order, which matches the order graph.Extract uses,
// so the index is consistent with what the graph saw. If a file lists
// the same handle twice, the first occurrence wins — duplicates within a
// single sidecar are a different lint signal (not yet implemented).
func buildHandleLocations(files []*File) map[handleSiteKey]Range {
	out := make(map[handleSiteKey]Range)
	for _, f := range files {
		feature := f.Tree.RootNode().ChildByFieldName("feature")
		if feature == nil {
			continue
		}
		for i := uint(0); i < feature.NamedChildCount(); i++ {
			block := feature.NamedChild(i)
			switch block.Kind() {
			case "provides_block", "consumes_block":
				indexHandleBlock(out, f, block)
			}
		}
	}
	return out
}

func indexHandleBlock(out map[handleSiteKey]Range, f *File, block *sitter.Node) {
	for i := uint(0); i < block.NamedChildCount(); i++ {
		entry := block.NamedChild(i)
		if entry.Kind() != "handle_entry" {
			continue
		}
		handleNode := entry.ChildByFieldName("handle")
		if handleNode == nil {
			continue
		}
		h, ok := graph.ExtractHandle(handleNode, f.Source)
		if !ok {
			continue
		}
		key := handleSiteKey{Path: f.Path, Handle: h}
		if _, exists := out[key]; exists {
			continue // first occurrence wins
		}
		out[key] = RangeFromNode(entry)
	}
}
