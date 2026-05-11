package cli

import "github.com/spf13/cobra"

// NewRootCmd builds a fresh trellis cobra command tree. Each call returns a
// new root with its own subcommand instances — tests can drive the root
// directly via SetArgs/SetOut without affecting other tests.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "trellis",
		Short:         "Tooling for .trellis sidecar specification files",
		Long:          "Trellis is a sidecar specification format. The CLI exposes graph, lint, and language-server capabilities as subcommands sharing a single binary.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newGraphCmd())
	root.AddCommand(newLintCmd())
	root.AddCommand(newLspCmd())
	return root
}

// Execute is the production entry point. It builds the root, runs it
// against os.Args, and returns the resulting error to main for exit-code
// handling.
func Execute() error {
	return NewRootCmd().Execute()
}
