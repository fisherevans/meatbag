package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fisherevans/meatbag/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date",
		RunE: func(cmd *cobra.Command, args []string) error {
			if gFlags.JSON {
				return emitJSON(map[string]string{
					"version": version.Version,
					"commit":  version.Commit,
					"date":    version.Date,
				})
			}
			fmt.Println(version.String())
			return nil
		},
	}
}
