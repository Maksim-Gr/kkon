package task

import (
	"fmt"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart a task",
	Long:  "Restarts a single Kafka Connect task (select interactively or use --connector and --id).",
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
			color.Yellow("[dry-run] Would restart %s\n", util.FormatTaskRef(name, id))
			return
		}

		var confirm bool
		confirmPrompt := &survey.Confirm{
			Message: fmt.Sprintf("Restart %s?", util.FormatTaskRef(name, id)),
			Default: true,
		}
		if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
			color.Yellow("Canceled\n")
			return
		}
		if !confirm {
			color.Yellow("Canceled\n")
			return
		}

		if err := client.RestartConnectorTask(cmd.Context(), name, id); err != nil {
			color.Red("Failed to restart %s: %v\n", util.FormatTaskRef(name, id), err)
			return
		}

		color.Green("Restart requested for %s\n", util.FormatTaskRef(name, id))
	},
}

func init() {
	Cmd.AddCommand(restartCmd)
}
