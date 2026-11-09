package schema

import (
	"fmt"

	"github.com/Maksim-Gr/kkon/internal/util"
	"github.com/spf13/cobra"
)

func getCommand() *cobra.Command {
	var version string
	cmd := &cobra.Command{
		Use: "get [subject]", Short: "Retrieve schema content (JSON output includes registry metadata)", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := registryClient()
			if err != nil {
				return err
			}
			subject, err := selectSubject(cmd, client, args, false)
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), subject, version)
			if err != nil {
				return err
			}
			if util.IsJSONOutput(cmd) {
				return printJSON(cmd, result)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), result.Schema.Schema)
			return err
		},
	}
	cmd.Flags().StringVar(&version, "version", "latest", "Registered version number or latest")
	return cmd
}
