package cli

import "github.com/spf13/cobra"

// Root returns the top-level cobra command for the meatbag binary. Subcommand
// wiring lives in this package's other files (list.go, item.go, ...).
func Root() *cobra.Command {
	return newRoot()
}
