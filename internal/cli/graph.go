package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/norlinga/trellis/internal/graph"
	"github.com/norlinga/trellis/internal/parser"
)

func newGraphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Build and query the .trellis dependency graph",
	}
	cmd.AddCommand(
		newGraphParseCmd(),
		newGraphBuildCmd(),
		newGraphDepsCmd(),
		newGraphDependentsCmd(),
		newGraphDownstreamCmd(),
		newGraphOrphansCmd(),
	)
	return cmd
}

// `trellis graph parse <file>` — parser-wiring smoke command. Kept around
// because it's the smallest reproducer when the binding or grammar drifts.
func newGraphParseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "parse <file>",
		Short: "Parse a single .trellis file and print its S-expression tree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tree, _, err := parser.ParseFile(args[0])
			if err != nil {
				return err
			}
			defer tree.Close()
			root := tree.RootNode()
			if root.HasError() {
				return fmt.Errorf("parse errors in %s", args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), root.ToSexp())
			return nil
		},
	}
}

func newGraphBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build [path...]",
		Short: "Discover .trellis files, build the graph, print a summary",
		Long: "Walks each given path (default: current directory) for .trellis files, " +
			"parses them, and prints a summary of sidecars, edges, and unresolved consumes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			roots := args
			if len(roots) == 0 {
				roots = []string{"."}
			}
			res, err := graph.Load(roots...)
			if err != nil {
				return err
			}
			printSummary(cmd.OutOrStdout(), res)
			return nil
		},
	}
}

func newGraphDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps <sidecar> [path...]",
		Short: "Show what a sidecar consumes (resolved against the workspace)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			roots := args[1:]
			if len(roots) == 0 {
				roots = []string{"."}
			}
			res, err := graph.Load(roots...)
			if err != nil {
				return err
			}
			sc, err := lookupOrLoad(res.Graph, target)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			seen := make(map[string]bool)
			for _, e := range res.Graph.Deps(sc) {
				key := e.Handle.String() + "→" + e.To.Path
				if seen[key] {
					continue
				}
				seen[key] = true
				fmt.Fprintf(out, "%s → %s\n", e.Handle, relPath(e.To.Path))
			}
			for _, c := range sc.Consumes {
				if hasResolvedConsume(res.Graph, sc, c.Handle) {
					continue
				}
				fmt.Fprintf(out, "%s → (unresolved)\n", c.Handle)
			}
			return nil
		},
	}
	return cmd
}

func newGraphDependentsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dependents <sidecar> [path...]",
		Short: "Show which sidecars consume something this one provides",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			roots := args[1:]
			if len(roots) == 0 {
				roots = []string{"."}
			}
			res, err := graph.Load(roots...)
			if err != nil {
				return err
			}
			sc, err := lookupOrLoad(res.Graph, target)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			edges := res.Graph.Dependents(sc)
			sort.Slice(edges, func(i, j int) bool {
				if edges[i].From.Path != edges[j].From.Path {
					return edges[i].From.Path < edges[j].From.Path
				}
				return edges[i].Handle.String() < edges[j].Handle.String()
			})
			for _, e := range edges {
				fmt.Fprintf(out, "%s ← %s\n", e.Handle, relPath(e.From.Path))
			}
			return nil
		},
	}
}

func newGraphDownstreamCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "downstream <sidecar> [path...]",
		Short: "Show every sidecar that transitively depends on this one",
		Long: "Whitepaper §4.1 blast-radius query: 'what would break if I changed this " +
			"signature?' Returns the transitive closure of dependents, BFS order, " +
			"closer dependents first.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			roots := args[1:]
			if len(roots) == 0 {
				roots = []string{"."}
			}
			res, err := graph.Load(roots...)
			if err != nil {
				return err
			}
			sc, err := lookupOrLoad(res.Graph, target)
			if err != nil {
				return err
			}
			for _, d := range res.Graph.Downstream(sc) {
				fmt.Fprintln(cmd.OutOrStdout(), relPath(d.Path))
			}
			return nil
		},
	}
}

func newGraphOrphansCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "orphans [path...]",
		Short: "List sidecars whose Provides handles have no consumers in the workspace",
		Long: "Whitepaper §4.1 'unreferenced provides' check: a sidecar appears here when " +
			"every handle in its Provides list has zero inbound consumers. Empty Provides " +
			"is intentionally not flagged — that's a separate lint signal.",
		RunE: func(cmd *cobra.Command, args []string) error {
			roots := args
			if len(roots) == 0 {
				roots = []string{"."}
			}
			res, err := graph.Load(roots...)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, sc := range res.Graph.Orphans() {
				fmt.Fprintln(out, relPath(sc.Path))
			}
			return nil
		},
	}
}

// lookupOrLoad first looks up the sidecar by absolute path in the loaded
// graph; if missing, it falls back to walking by suffix match on filename.
// This is the ergonomic concession that lets `trellis graph deps foo.rb.trellis`
// work without forcing the user to type the full path.
func lookupOrLoad(g *graph.Graph, target string) (*graph.Sidecar, error) {
	abs, err := filepath.Abs(target)
	if err == nil {
		if sc := g.Lookup(abs); sc != nil {
			return sc, nil
		}
	}
	var matches []*graph.Sidecar
	for _, sc := range g.Sidecars {
		if sc.Path == target || filepath.Base(sc.Path) == target {
			matches = append(matches, sc)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("sidecar %q not found in workspace", target)
	case 1:
		return matches[0], nil
	default:
		paths := make([]string, len(matches))
		for i, m := range matches {
			paths[i] = m.Path
		}
		sort.Strings(paths)
		return nil, fmt.Errorf("sidecar %q is ambiguous; candidates:\n  %s", target, joinLines(paths))
	}
}

func hasResolvedConsume(g *graph.Graph, sc *graph.Sidecar, h graph.Handle) bool {
	for _, e := range g.Deps(sc) {
		if e.Handle == h {
			return true
		}
	}
	return false
}

func printSummary(w io.Writer, res *graph.LoadResult) {
	g := res.Graph
	fmt.Fprintf(w, "sidecars:        %d\n", len(g.Sidecars))
	fmt.Fprintf(w, "resolved edges:  %d\n", len(g.Edges))
	fmt.Fprintf(w, "unresolved:      %d\n", len(g.Unresolved))
	if dups := g.DuplicateProvides(); len(dups) > 0 {
		fmt.Fprintf(w, "duplicate provides: %d\n", len(dups))
		handles := make([]string, 0, len(dups))
		for h := range dups {
			handles = append(handles, h.String())
		}
		sort.Strings(handles)
		for _, hs := range handles {
			fmt.Fprintf(w, "  %s\n", hs)
		}
	}
	if len(res.ParseErrs) > 0 {
		fmt.Fprintf(w, "parse errors:    %d\n", len(res.ParseErrs))
		for _, pe := range res.ParseErrs {
			fmt.Fprintf(w, "  %s: %v\n", relPath(pe.Path), pe.Err)
		}
	}
	if len(g.Unresolved) > 0 {
		fmt.Fprintln(w, "unresolved consumes:")
		for _, u := range g.Unresolved {
			fmt.Fprintf(w, "  %s ← %s\n", u.Handle, relPath(u.From.Path))
		}
	}
}

func relPath(p string) string {
	if cwd, err := filepath.Abs("."); err == nil {
		if rel, err := filepath.Rel(cwd, p); err == nil && len(rel) < len(p) {
			return rel
		}
	}
	return p
}

func joinLines(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += "\n  "
		}
		out += s
	}
	return out
}
