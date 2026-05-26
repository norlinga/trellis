package cli

import (
	"github.com/spf13/cobra"

	"github.com/norlinga/trellis/internal/lsp"
)

func newLspCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "Run the Trellis Language Server over stdio",
		Long: "Starts an LSP server on stdin/stdout. Editors that support LSP (Neovim, " +
			"VS Code with a wrapper extension, Helix, Zed, etc.) can spawn this command " +
			"and consume diagnostics for .trellis files. Blocks until the client sends " +
			"`exit`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return lsp.New().Run()
		},
	}
	// --stdio is a conventional no-op flag that LSP clients (e.g. vscode-languageclient)
	// pass to signal stdio transport. trellis lsp always uses stdio, so we accept and ignore it.
	cmd.Flags().Bool("stdio", false, "use stdio transport (default and only mode; accepted for LSP client compatibility)")
	return cmd
}
