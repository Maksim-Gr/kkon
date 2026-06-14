// Package task provides CLI commands for managing Kafka Connect tasks.
package task

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get task status",
	Long:  "Fetches status for a single task (select interactively or use --connector and --id).",
	Run: func(cmd *cobra.Command, _ []string) {
		client, ok := util.NewKafkaConnectClient()
		if !ok {
			return
		}

		name, ok := util.ResolveConnectorName(cmd.Context(), client, connectorName)
		if !ok {
			return
		}

		isDryRun := dryRun != nil && *dryRun
		id, ok := util.ResolveTaskID(cmd.Context(), client, name, taskID, isDryRun)
		if !ok {
			return
		}

		if isDryRun {
			color.Yellow("[dry-run] Would get status for %s\n", util.FormatTaskRef(name, id))
			return
		}

		jsonMode := cmd.Root().PersistentFlags().Lookup("output").Value.String() == "json"

		status, err := client.GetConnectorTaskStatus(cmd.Context(), name, id)
		if err != nil {
			if jsonMode {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			} else {
				color.Red("Failed to get status for %s: %v\n", util.FormatTaskRef(name, id), err)
			}
			return
		}

		if jsonMode {
			b, _ := json.MarshalIndent(status, "", "  ")
			fmt.Println(string(b))
			return
		}

		color.Cyan("Task status:")
		fmt.Printf("\tConnector: %s\n", name)
		fmt.Printf("\tTask ID:   %d\n", status.ID)
		fmt.Printf("\tState:     %s\n", util.ColorState(status.State))
		fmt.Printf("\tWorker:    %s\n", status.WorkerID)
		if status.Trace != "" {
			color.Yellow("\tTrace:\n%s\n", status.Trace)
		}
	},
}

func init() {
	Cmd.AddCommand(getCmd)
}
