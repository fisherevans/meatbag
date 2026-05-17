package cli

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed agent_help.md
var agentGuide string

//go:embed agent_snippet.md
var agentSnippet string

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
	cmd.AddCommand(&cobra.Command{
		Use:   "snippet",
		Short: "Print a short markdown snippet to paste into CLAUDE.md / AGENTS.md",
		Long: "Print a short, model-agnostic markdown snippet that introduces " +
			"meatbag to an LLM agent. Paste it into your project's CLAUDE.md, " +
			"AGENTS.md, .cursorrules, or equivalent agent config file. With " +
			"--json the snippet is wrapped as {\"snippet\": \"...\"} so it can " +
			"be piped through `jq -r .snippet >> CLAUDE.md`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if gFlags.JSON {
				return emitJSON(map[string]string{"snippet": agentSnippet})
			}
			fmt.Print(agentSnippet)
			return nil
		},
	})
	return cmd
}
