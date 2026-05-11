package lint

import (
	"github.com/norlinga/trellis/internal/graph"
	"github.com/norlinga/trellis/internal/parser"
)

// LintSingleFile parses source, extracts the Sidecar, runs the per-file
// rule set, and returns the diagnostics. The tree-sitter Tree is closed
// before return — this entry point is intended for short-lived contexts
// like an LSP edit cycle, where the caller doesn't want to manage Close.
//
// If the parse contains ERROR nodes, returns a single synthetic
// `parse-error` diagnostic anchored to the file start. Per-file rules
// don't run on broken trees — most rules assume a well-formed AST and
// would either crash or produce noise.
//
// For workspace-aware linting (broken-link, duplicate-provides,
// orphan-source-file) use LoadWorkspace + Linter.Lint instead.
func LintSingleFile(path string, src []byte) []Diagnostic {
	tree, err := parser.Parse(src)
	if err != nil {
		return []Diagnostic{{
			Code:     "parse-failed",
			Severity: SeverityError,
			Message:  "parser error: " + err.Error(),
			Path:     path,
			Range:    FileStart(),
		}}
	}
	defer tree.Close()

	if tree.RootNode().HasError() {
		return []Diagnostic{{
			Code:     "parse-error",
			Severity: SeverityError,
			Message:  "the file contains syntax errors that prevent further analysis",
			Path:     path,
			Range:    FileStart(),
		}}
	}

	sc, err := graph.Extract(tree, src, path)
	if err != nil {
		return []Diagnostic{{
			Code:     "extract-failed",
			Severity: SeverityError,
			Message:  "failed to extract sidecar: " + err.Error(),
			Path:     path,
			Range:    FileStart(),
		}}
	}

	f := &File{
		Path:        path,
		Source:      src,
		Tree:        tree,
		Sidecar:     sc,
		Overrides:   extractOverrides(tree, src),
		Frontmatter: extractFrontmatter(tree, src),
	}
	// Run only per-file rules — workspace rules need a graph that this
	// entry point has no way to construct.
	return PerFileLinter().Lint([]*File{f}, nil)
}
