package graph

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/norlinga/trellis/internal/parser"
)

// LoadResult bundles the graph with per-file diagnostics. Parse failures do
// not abort the load — the workspace is reported in full so downstream
// tools can render every problem at once instead of only the first.
type LoadResult struct {
	Graph     *Graph
	ParseErrs []ParseError
}

type ParseError struct {
	Path string
	Err  error
}

func (e ParseError) Error() string { return fmt.Sprintf("%s: %v", e.Path, e.Err) }

// Load discovers .trellis files under each root, parses them, extracts
// Sidecars, and builds the graph. Roots may be files (loaded directly) or
// directories (walked recursively). A nil/empty roots argument returns an
// empty graph.
//
// Discovery rules:
//   - Only paths ending in `.trellis` are considered.
//   - Hidden directories (leading `.`) are skipped — vendored caches like
//     `.git` and `.cache` would otherwise dominate walks of large repos.
//   - Symlinks are not followed (filepath.WalkDir behavior).
//   - The order of returned Sidecars is deterministic (sorted by path) so
//     CLI output and test expectations stay stable.
func Load(roots ...string) (*LoadResult, error) {
	paths, err := DiscoverTrellisFiles(roots)
	if err != nil {
		return nil, err
	}
	res := &LoadResult{}
	var sidecars []*Sidecar
	for _, p := range paths {
		tree, src, err := parser.ParseFile(p)
		if err != nil {
			res.ParseErrs = append(res.ParseErrs, ParseError{Path: p, Err: err})
			continue
		}
		root := tree.RootNode()
		if root.HasError() {
			tree.Close()
			res.ParseErrs = append(res.ParseErrs, ParseError{Path: p, Err: errors.New("parse contains ERROR nodes")})
			continue
		}
		sc, err := Extract(tree, src, p)
		tree.Close()
		if err != nil {
			res.ParseErrs = append(res.ParseErrs, ParseError{Path: p, Err: err})
			continue
		}
		sidecars = append(sidecars, sc)
	}
	res.Graph = Build(sidecars)
	return res, nil
}

// DiscoverTrellisFiles walks each root for .trellis files, applying the
// same rules Load uses (skip hidden directories, no symlink follow, sorted
// output, deduplicated by absolute path). Exposed for callers — like the
// linter — that need the same discovery shape but parse files differently.
func DiscoverTrellisFiles(roots []string) ([]string, error) {
	seen := make(map[string]bool)
	var out []string
	add := func(p string) {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if seen[abs] {
			return
		}
		seen[abs] = true
		out = append(out, abs)
	}
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", root, err)
		}
		if !info.IsDir() {
			if strings.HasSuffix(root, ".trellis") {
				add(root)
			}
			continue
		}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path != root && strings.HasPrefix(d.Name(), ".") {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(d.Name(), ".trellis") {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", root, err)
		}
	}
	sort.Strings(out)
	return out, nil
}
