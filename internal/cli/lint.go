package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/norlinga/trellis/internal/lint"
)

func newLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [path...]",
		Short: "Run lint rules over .trellis sidecars",
		Long: "Walks each given path (default: current directory), parses every .trellis " +
			"file, and runs the default rule set. Emits one diagnostic per line as " +
			"'path:line:col: severity [code] message'. Exits 1 if any error-severity " +
			"diagnostic was emitted; warnings alone do not affect the exit code.",
		RunE: func(cmd *cobra.Command, args []string) error {
			roots := args
			if len(roots) == 0 {
				roots = []string{"."}
			}
			ws, err := lint.LoadWorkspace(roots...)
			if err != nil {
				return err
			}
			defer ws.Close()

			linter := lint.Default()
			linter.AddPolicyRules(ws.Policy)
			diags := linter.Lint(ws.Files, ws.Graph)
			// Make paths relative to cwd for ergonomics; the formatter
			// passes them through unchanged.
			for i := range diags {
				diags[i].Path = relPath(diags[i].Path)
			}
			lint.SortDiagnostics(diags)
			out := cmd.OutOrStdout()
			lint.Format(out, diags)

			// Surface parse errors below the diagnostics so they don't
			// drown lint output. Each parse error counts as an error for
			// exit-code purposes — a file we couldn't even parse is more
			// urgent than any lint warning.
			for _, pe := range ws.ParseErrs {
				fmt.Fprintf(out, "%s:1:1: error [parse-failed] %v\n", relPath(pe.Path), pe.Err)
			}
			// Policy file parse errors share the parse-failed channel —
			// they're a different kind of input but the same shape of
			// "we couldn't read this artifact you committed."
			for _, pe := range ws.PolicyErrs {
				line := pe.Line
				if line < 1 {
					line = 1
				}
				fmt.Fprintf(out, "%s:%d:1: error [policy-parse-failed] %s\n", relPath(pe.Path), line, pe.Message)
			}

			summary := lint.Summarize(diags)
			extraErrs := len(ws.ParseErrs) + len(ws.PolicyErrs)
			fmt.Fprintf(out, "\n%d error(s), %d warning(s)", summary.Errors+extraErrs, summary.Warnings)
			if summary.Infos > 0 {
				fmt.Fprintf(out, ", %d info", summary.Infos)
			}
			fmt.Fprintln(out)

			if summary.Errors > 0 || extraErrs > 0 {
				return errors.New("lint errors found")
			}
			return nil
		},
	}
}

// relPath is defined in graph.go and reused here for consistent path display.
