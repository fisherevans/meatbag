package cli

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed agent_help.md
var agentGuide string

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent-facing usage guide",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "help",
		Short: "Print the full markdown guide for LLM agents using meatbag",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(agentGuide)
			return nil
		},
	})
	return cmd
}
