package task

import (
	"context"
	"errors"

	"github.com/Maksim-Gr/kkon/cmd/action"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/spf13/cobra"
)

var restartYes bool

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart a task",
	Long: `Restart a single task of a connector, with confirmation (skip it with
--yes). Prompts for the connector and task when --connector or --id are not
given.`,
	Example: `  # Pick a connector and task interactively
  kkon task restart

  # Restart a specific task
  kkon task restart --connector my-connector --id 1

  # Restart without the confirmation prompt (for scripting)
  kkon task restart --connector my-connector --id 1 --yes

  # Preview without restarting
  kkon task restart --connector my-connector --id 1 --dry-run`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		jsonMode := util.IsJSONOutput(cmd)
		if jsonMode && (connectorName == "" || taskID < 0 || !restartYes) {
			return errors.New("--output json requires non-interactive usage (pass --connector, --id and --yes)")
		}

		client, err := util.NewKafkaConnectClient()
		if err != nil {
			return err
		}

		name, err := util.ResolveConnectorName(cmd.Context(), client, connectorName)
		if err != nil {
			return err
		}

		id, err := util.ResolveTaskID(cmd.Context(), client, name, taskID, util.IsDryRun(cmd))
		if err != nil {
			return err
		}

		return action.Run(cmd, action.Spec{
			Verb:           "restart",
			SuccessFmt:     "Restart requested for %s\n",
			ConfirmSingle:  true,
			ConfirmDefault: true,
			Op: func(ctx context.Context, t action.Target) error {
				return client.RestartConnectorTask(ctx, t.Connector, *t.Task)
			},
		}, []action.Target{{Connector: name, Task: &id}})
	},
}

func init() {
	restartCmd.Flags().BoolVarP(&restartYes, "yes", "y", false, "Skip the confirmation prompt (for scripting)")
	Cmd.AddCommand(restartCmd)
}
