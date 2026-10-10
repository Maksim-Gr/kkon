package connector

import (
	"errors"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/spf13/cobra"
)

// UpdateCmd interactively updates an existing connector's configuration.
var UpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update an existing connector's configuration",
	Long: `Edit a connector's live configuration interactively, review the changes,
and apply them. Pick the connector from a prompt or pass its name. This command is interactive and does not support
--output json.`,
	Example: `  # Pick a connector and edit its config
  kkon connector update

  # Edit a specific connector
  kkon connector update my-connector

  # Preview the edit without applying it
  kkon connector update --dry-run`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if util.IsJSONOutput(cmd) {
			return errors.New("--output json is not supported for update (interactive command)")
		}

		client, err := util.NewKafkaConnectClient()
		if err != nil {
			return err
		}

		selected, err := util.ResolveConnectorName(cmd.Context(), client, argOrEmpty(args))
		if err != nil {
			return err
		}

		return editConnectorConfig(cmd.Context(), client, selected, util.IsDryRun(cmd))
	},
}
