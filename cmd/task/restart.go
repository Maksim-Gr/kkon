package task

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
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

		isDryRun := util.IsDryRun(cmd)
		id, err := util.ResolveTaskID(cmd.Context(), client, name, taskID, isDryRun)
		if err != nil {
			return err
		}

		if isDryRun {
			if jsonMode {
				printRestartJSON(name, id, "dry-run")
				return nil
			}
			color.Yellow("[dry-run] Would restart %s\n", util.FormatTaskRef(name, id))
			return nil
		}

		if !restartYes {
			var confirm bool
			confirmPrompt := &survey.Confirm{
				Message: fmt.Sprintf("Restart %s?", util.FormatTaskRef(name, id)),
				Default: true,
			}
			if err := survey.AskOne(confirmPrompt, &confirm); err != nil || !confirm {
				return util.ErrCanceled
			}
		}

		if err := client.RestartConnectorTask(cmd.Context(), name, id); err != nil {
			return fmt.Errorf("failed to restart %s: %w", util.FormatTaskRef(name, id), err)
		}

		if jsonMode {
			printRestartJSON(name, id, "ok")
			return nil
		}
		color.Green("Restart requested for %s\n", util.FormatTaskRef(name, id))
		return nil
	},
}

func init() {
	restartCmd.Flags().BoolVarP(&restartYes, "yes", "y", false, "Skip the confirmation prompt (for scripting)")
	Cmd.AddCommand(restartCmd)
}

// printRestartJSON matches the connector lifecycle JSON shape, plus the task id.
func printRestartJSON(name string, id int, result string) {
	b, _ := json.Marshal(map[string]any{"name": name, "task": id, "action": "restart", "result": result})
	fmt.Println(string(b))
}
