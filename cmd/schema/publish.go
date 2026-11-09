package schema

import (
	"fmt"
	"strconv"

	"github.com/Maksim-Gr/kkon/internal/schemaregistry"
	"github.com/Maksim-Gr/kkon/internal/util"
	"github.com/spf13/cobra"
)

func publish(cmd *cobra.Command, client *schemaregistry.Client, subject string, input schemaregistry.Schema) error {
	if util.IsDryRun(cmd) {
		if util.IsJSONOutput(cmd) {
			return printJSON(cmd, struct {
				Subject string `json:"subject"`
				DryRun  bool   `json:"dry_run"`
				schemaregistry.Schema
			}{Subject: subject, DryRun: true, Schema: input})
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Would publish %s schema to subject %s (no registration or compatibility check performed):\n", input.SchemaType, strconv.Quote(subject)); err != nil {
			return err
		}
		return printJSON(cmd, input)
	}
	result, err := client.Publish(cmd.Context(), subject, input)
	if err != nil {
		return err
	}
	if util.IsJSONOutput(cmd) {
		return printJSON(cmd, struct {
			Subject string `json:"subject"`
			schemaregistry.Registration
		}{Subject: subject, Registration: result})
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Published schema ID %d to subject %s\n", result.ID, strconv.Quote(subject))
	return err
}
