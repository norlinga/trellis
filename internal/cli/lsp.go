package cli

import (
	"github.com/spf13/cobra"

	"github.com/norlinga/trellis/internal/lsp"
)

func newLspCmd() *cobra.Command {
	return &cobra.Command{
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
}
