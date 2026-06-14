package connector

import (
	"fmt"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	restartIncludeTasks bool
	restartOnlyFailed   bool
)

// RestartCmd restarts a connector.
var RestartCmd = &cobra.Command{
	Use:   "restart [name]",
	Short: "Restart a connector",
	Long:  "Restarts a Kafka Connect connector (select interactively or pass the connector name).",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, ok := util.NewKafkaConnectClient()
		if !ok {
			return
		}

		name, ok := util.ResolveConnectorName(cmd.Context(), client, argOrEmpty(args))
		if !ok {
			return
		}

		if isDryRun(cmd) {
			color.Yellow("[dry-run] Would restart connector %s (includeTasks=%t, onlyFailed=%t)\n",
				name, restartIncludeTasks, restartOnlyFailed)
			return
		}

		var confirm bool
		confirmPrompt := &survey.Confirm{
			Message: fmt.Sprintf("Restart connector %s?", name),
			Default: true,
		}
		if err := survey.AskOne(confirmPrompt, &confirm); err != nil || !confirm {
			color.Yellow("Canceled\n")
			return
		}

		if err := client.RestartConnector(cmd.Context(), name, restartIncludeTasks, restartOnlyFailed); err != nil {
			color.Red("Failed to restart %s: %v\n", name, err)
			return
		}
		color.Green("Restart requested for %s\n", name)
	},
}

func init() {
	RestartCmd.Flags().BoolVar(&restartIncludeTasks, "include-tasks", true, "Also restart the connector's tasks")
	RestartCmd.Flags().BoolVar(&restartOnlyFailed, "only-failed", false, "Restart only FAILED connector and tasks")
}
