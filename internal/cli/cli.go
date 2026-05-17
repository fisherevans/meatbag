package cli

import (
	"github.com/spf13/cobra"

	"github.com/fisherevans/meatbag/internal/version"
)

// globalFlags carries top-level options accessible to every subcommand.
type globalFlags struct {
	JSON  bool
	Quiet bool
	Home  string
}

var gFlags globalFlags

func newRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meatbag",
		Short: "Shared to-do lists for humans and LLM agents",
		Long: "meatbag is a local CLI + web UI for to-do lists. Agents create " +
			"and update lists via this CLI; humans work through them in a web " +
			"daemon at http://127.0.0.1:7421. Run `meatbag agent help` for the " +
			"agent-facing usage guide.",
		Version:      version.String(),
		SilenceUsage: true,
	}
	cmd.PersistentFlags().BoolVar(&gFlags.JSON, "json", false, "emit machine-readable JSON output")
	cmd.PersistentFlags().BoolVarP(&gFlags.Quiet, "quiet", "q", false, "reduce non-essential output")
	cmd.PersistentFlags().StringVar(&gFlags.Home, "home", "", "override data dir (default $MEATBAG_HOME or ~/.meatbag)")

	// Wire subcommands. Each file owns its subtree.
	cmd.AddCommand(
		newListCmd(),
		newItemCmd(),
		newInputCmd(),
		newURLCmd(),
		newWaitCmd(),
		newWebCmd(),
		newGCCmd(),
		newAgentCmd(),
		newInstallCmd(),
		newVersionCmd(),
	)
	return cmd
}
