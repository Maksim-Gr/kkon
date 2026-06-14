package task

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks for a connector",
	Long:  "Lists tasks for a selected connector (or --connector).",
	Run: func(cmd *cobra.Command, _ []string) {
		client, ok := util.NewKafkaConnectClient()
		if !ok {
			return
		}

		name, ok := util.ResolveConnectorName(cmd.Context(), client, connectorName)
		if !ok {
			return
		}

		if dryRun != nil && *dryRun {
			color.Yellow("[dry-run] Would list tasks for connector: %s\n", name)
			return
		}

		jsonMode := cmd.Root().PersistentFlags().Lookup("output").Value.String() == "json"

		stop := util.StartSpinner("Fetching tasks...")
		tasks, err := client.ListConnectorTasks(cmd.Context(), name)
		connStatus, _ := client.GetConnectorStatus(cmd.Context(), name)
		stop()

		if err != nil {
			if jsonMode {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			} else {
				color.Red("Failed to list tasks for %s: %v\n", name, err)
			}
			return
		}

		if jsonMode {
			type taskJSON struct {
				Connector string `json:"connector"`
				Task      int    `json:"task"`
				State     string `json:"state,omitempty"`
			}
			taskStates := make(map[int]string, len(connStatus.Tasks))
			for _, ts := range connStatus.Tasks {
				taskStates[ts.ID] = ts.State
			}
			out := make([]taskJSON, 0, len(tasks))
			for _, t := range tasks {
				e := taskJSON{Connector: t.Connector, Task: t.Task}
				if state, ok := taskStates[t.Task]; ok {
					e.State = state
				}
				out = append(out, e)
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(b))
			return
		}

		if len(tasks) == 0 {
			color.Yellow("No tasks found for %s\n", name)
			return
		}

		taskStates := make(map[int]string, len(connStatus.Tasks))
		for _, ts := range connStatus.Tasks {
			taskStates[ts.ID] = ts.State
		}

		color.Cyan("Tasks for %s:", name)
		for _, t := range tasks {
			badge := ""
			if state, ok := taskStates[t.Task]; ok {
				badge = "  " + util.ColorState(state)
			}
			fmt.Printf("  Task %d%s\n", t.Task, badge)
		}
	},
}

func init() {
	Cmd.AddCommand(listCmd)
}
