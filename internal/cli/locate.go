package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/norlinga/trellis/internal/graph"
)

type locateOutput struct {
	Handle      string             `json:"handle"`
	SidecarPath string             `json:"sidecar_path"`
	SourcePath  string             `json:"source_path"`
	Anchor      locateAnchorOutput `json:"anchor"`
}

type locateAnchorOutput struct {
	Value     string `json:"value"`
	Kind      string `json:"kind,omitempty"`
	Target    string `json:"target,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

func newLocateCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "locate <handle> [path...]",
		Short: "Resolve a provided handle to its source anchor",
		Long: "Finds the sidecar that Provides the exact handle and prints the paired source " +
			"file plus its @source(...) anchor. Paths default to the current directory.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, ok := graph.ParseHandleLiteral(args[0])
			if !ok {
				return fmt.Errorf("invalid handle %q", args[0])
			}
			roots := args[1:]
			if len(roots) == 0 {
				roots = []string{"."}
			}
			res, err := graph.Load(roots...)
			if err != nil {
				return err
			}
			out, err := locateHandle(res.Graph, h)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s%s\n", out.Handle, relPath(out.SourcePath), renderAnchorSuffix(out.Anchor))
			fmt.Fprintf(cmd.OutOrStdout(), "sidecar: %s\n", relPath(out.SidecarPath))
			fmt.Fprintf(cmd.OutOrStdout(), "anchor:  %s\n", out.Anchor.Value)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON output")
	return cmd
}

func locateHandle(g *graph.Graph, h graph.Handle) (*locateOutput, error) {
	providers := g.ProvidersOf(h)
	switch len(providers) {
	case 0:
		return nil, fmt.Errorf("handle %s is not provided in this workspace", h)
	case 1:
	default:
		return nil, fmt.Errorf("handle %s has %d providers; run trellis lint to resolve duplicate-provides", h, len(providers))
	}
	sc := providers[0]
	entry, ok := providedEntry(sc, h)
	if !ok {
		return nil, fmt.Errorf("provider for %s did not retain its Provides entry", h)
	}
	if entry.SourceAnchor == nil {
		return nil, fmt.Errorf("provider %s has no source anchor for %s", relPath(sc.Path), h)
	}
	a := entry.SourceAnchor
	return &locateOutput{
		Handle:      h.String(),
		SidecarPath: sc.Path,
		SourcePath:  sc.SourcePath,
		Anchor: locateAnchorOutput{
			Value:     a.Value,
			Kind:      a.Kind,
			Target:    a.Target,
			StartLine: a.StartLine,
			EndLine:   a.EndLine,
		},
	}, nil
}

func providedEntry(sc *graph.Sidecar, h graph.Handle) (graph.Entry, bool) {
	for _, entry := range sc.Provides {
		if entry.Handle == h {
			return entry, true
		}
	}
	return graph.Entry{}, false
}

func renderAnchorSuffix(a locateAnchorOutput) string {
	switch {
	case a.Kind == "line" && a.StartLine > 0 && a.EndLine > 0 && a.StartLine == a.EndLine:
		return fmt.Sprintf(":%d", a.StartLine)
	case a.Kind == "line" && a.StartLine > 0 && a.EndLine > 0:
		return fmt.Sprintf(":%d-%d", a.StartLine, a.EndLine)
	case a.Target != "":
		return "#" + a.Target
	case a.Value != "":
		return "#" + filepath.Base(a.Value)
	default:
		return ""
	}
}
