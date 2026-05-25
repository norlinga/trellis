package graph

import "strings"

// Entry is one item in a Provides: or Consumes: list. The handle is the
// structured part the graph cares about; the source anchor is optional
// implementation-location metadata; the description is preserved verbatim
// for hover docs and lint diagnostics, never interpreted.
type Entry struct {
	Handle       Handle
	SourceAnchor *SourceAnchor
	Description  string // verbatim, may be empty
}

// SourceAnchor points from a Trellis handle to an implementation location in
// the paired source file. Value is the exact quoted payload from @source("...").
// Parsed fields are best-effort conveniences for CLI/LSP consumers; Value
// remains the stable round-tripped representation.
type SourceAnchor struct {
	Value  string
	Kind   string
	Target string

	StartLine int
	EndLine   int
}

// Sidecar is one parsed .trellis file as the graph sees it. Linter-only
// concerns (Invariants, OutOfScope, frontmatter) are deliberately omitted
// — they're not load-bearing for graph construction.
type Sidecar struct {
	Path           string // absolute path to the .trellis file
	SourcePath     string // implied source-file path (Path with .trellis suffix removed)
	FeatureName    string
	FeatureSummary string

	Provides []Entry
	Consumes []Entry
}

// SourcePathFor returns the implied source-file path for a sidecar path,
// per the convention `<source>.<ext>.trellis`. Returns an empty string if
// the path doesn't end in `.trellis` (caller should treat as malformed).
func SourcePathFor(sidecarPath string) string {
	const suffix = ".trellis"
	if !strings.HasSuffix(sidecarPath, suffix) {
		return ""
	}
	return strings.TrimSuffix(sidecarPath, suffix)
}
