package schema

import (
	"fmt"

	"github.com/Maksim-Gr/kkon/internal/schemaregistry"
	"github.com/Maksim-Gr/kkon/internal/util"
	"github.com/spf13/cobra"
)

func check(cmd *cobra.Command, client *schemaregistry.Client, subject string, input schemaregistry.Schema) error {
	result, err := client.Check(cmd.Context(), subject, input)
	if err != nil {
		return err
	}
	if util.IsJSONOutput(cmd) {
		if err := printJSON(cmd, result); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Compatible: %t\n", result.IsCompatible); err != nil {
			return err
		}
		for _, message := range result.Messages {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), message); err != nil {
				return err
			}
		}
	}
	if !result.IsCompatible {
		return fmt.Errorf("schema is incompatible with subject %q under its configured policy", subject)
	}
	return nil
}
