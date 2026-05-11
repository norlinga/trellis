package lint

import (
	"errors"

	"github.com/norlinga/trellis/internal/graph"
	"github.com/norlinga/trellis/internal/parser"
)

// Workspace holds the parsed Files plus the resolved Graph for a lint
// pass. Files retain their tree-sitter Trees so rules can walk the AST;
// Close releases the C-side memory and must be called when done.
//
// Policy is the merged result of every `*.trellis-policy` file
// discovered under the same roots as the .trellis files. Always
// non-nil — empty when no policy files exist or all of them failed to
// parse. PolicyErrs surfaces parse failures so the CLI can render them
// alongside lint diagnostics.
type Workspace struct {
	Files      []*File
	Graph      *graph.Graph
	Policy     *Policy
	ParseErrs  []graph.ParseError
	PolicyErrs []PolicyParseError
}

// Close releases tree-sitter Tree memory. Safe to call multiple times.
func (ws *Workspace) Close() {
	for _, f := range ws.Files {
		if f.Tree != nil {
			f.Tree.Close()
			f.Tree = nil
		}
	}
}

// LoadWorkspace discovers, parses, and extracts every .trellis file
// reachable from roots, returning a Workspace ready to lint. Files with
// parse errors do not abort the load — they appear in ParseErrs and are
// excluded from Files and from the graph.
//
// Caller MUST Close the returned Workspace when finished.
func LoadWorkspace(roots ...string) (*Workspace, error) {
	paths, err := graph.DiscoverTrellisFiles(roots)
	if err != nil {
		return nil, err
	}
	var (
		files     []*File
		sidecars  []*graph.Sidecar
		parseErrs []graph.ParseError
	)
	for _, p := range paths {
		tree, src, err := parser.ParseFile(p)
		if err != nil {
			parseErrs = append(parseErrs, graph.ParseError{Path: p, Err: err})
			continue
		}
		if tree.RootNode().HasError() {
			tree.Close()
			parseErrs = append(parseErrs, graph.ParseError{Path: p, Err: errors.New("parse contains ERROR nodes")})
			continue
		}
		sc, err := graph.Extract(tree, src, p)
		if err != nil {
			tree.Close()
			parseErrs = append(parseErrs, graph.ParseError{Path: p, Err: err})
			continue
		}
		files = append(files, &File{
			Path:        p,
			Source:      src,
			Tree:        tree,
			Sidecar:     sc,
			Overrides:   extractOverrides(tree, src),
			Frontmatter: extractFrontmatter(tree, src),
		})
		sidecars = append(sidecars, sc)
	}

	policy, policyErrs := loadPolicies(roots)

	return &Workspace{
		Files:      files,
		Graph:      graph.Build(sidecars),
		Policy:     policy,
		ParseErrs:  parseErrs,
		PolicyErrs: policyErrs,
	}, nil
}

// loadPolicies discovers every .trellis-policy file under roots, parses
// each, and returns the merged result. Per-file parse errors are
// accumulated; an unparseable file contributes no rules to the merge
// but does not abort the load (matches the .trellis behavior in the
// loop above).
func loadPolicies(roots []string) (*Policy, []PolicyParseError) {
	paths, err := DiscoverPolicyFiles(roots)
	if err != nil {
		return &Policy{}, []PolicyParseError{{Path: "", Line: 0, Message: err.Error()}}
	}
	var (
		policies []*Policy
		errs     []PolicyParseError
	)
	for _, p := range paths {
		pol, perrs := LoadPolicyFile(p)
		policies = append(policies, pol)
		errs = append(errs, perrs...)
	}
	return MergePolicies(policies), errs
}
