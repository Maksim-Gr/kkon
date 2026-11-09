package schema

import (
	"fmt"

	"github.com/Maksim-Gr/kkon/internal/util"
	"github.com/spf13/cobra"
)

func listCommand() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List registered subjects", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := registryClient()
			if err != nil {
				return err
			}
			subjects, err := client.List(cmd.Context())
			if err != nil {
				return err
			}
			if util.IsJSONOutput(cmd) {
				return printJSON(cmd, subjects)
			}
			for _, subject := range subjects {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), subject); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
