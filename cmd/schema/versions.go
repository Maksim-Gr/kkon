package schema

import (
	"fmt"

	"github.com/Maksim-Gr/kkon/internal/util"
	"github.com/spf13/cobra"
)

func versionsCommand() *cobra.Command {
	return &cobra.Command{
		Use: "versions [subject]", Short: "List a subject's registered versions", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := registryClient()
			if err != nil {
				return err
			}
			subject, err := selectSubject(cmd, client, args, false)
			if err != nil {
				return err
			}
			versions, err := client.Versions(cmd.Context(), subject)
			if err != nil {
				return err
			}
			if util.IsJSONOutput(cmd) {
				return printJSON(cmd, versions)
			}
			for _, version := range versions {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), version); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
