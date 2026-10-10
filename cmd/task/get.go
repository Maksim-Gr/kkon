// Package task provides CLI commands for managing Kafka Connect tasks.
package task

import (
	"encoding/json"
	"fmt"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the status of a single task",
	Long: `Fetch the state, worker, and error trace of one task. Prompts for the
connector and task when --connector or --id are not given.`,
	Example: `  # Pick a connector and task interactively
  kkon task get

  # Get the status of a specific task
  kkon task get --connector my-connector --id 0

  # Machine-readable output
  kkon task get --connector my-connector --id 0 --output json`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := util.NewKafkaConnectClient()
		if err != nil {
			return err
		}

		name, err := util.ResolveConnectorName(cmd.Context(), client, connectorName)
		if err != nil {
			return err
		}

		// Read-only: --dry-run is ignored, like the other read commands.
		id, err := util.ResolveTaskID(cmd.Context(), client, name, taskID, false)
		if err != nil {
			return err
		}

		status, err := client.GetConnectorTaskStatus(cmd.Context(), name, id)
		if err != nil {
			return fmt.Errorf("failed to get status for %s: %w", util.FormatTaskRef(name, id), err)
		}

		if util.IsJSONOutput(cmd) {
			b, _ := json.MarshalIndent(status, "", "  ")
			fmt.Println(string(b))
			return nil
		}

		color.Cyan("Task status:")
		fmt.Printf("\tConnector: %s\n", name)
		fmt.Printf("\tTask ID:   %d\n", status.ID)
		fmt.Printf("\tState:     %s\n", util.ColorState(status.State))
		fmt.Printf("\tWorker:    %s\n", status.WorkerID)
		if status.Trace != "" {
			color.Yellow("\tTrace:\n%s\n", status.Trace)
		}
		return nil
	},
}

func init() {
	Cmd.AddCommand(getCmd)
}
