package connector

import (
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var deleteYes bool

// DeleteCmd represents the delete command.
var DeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a connector",
	Long:  "Delete a connector from Kafka Connect (select interactively or pass the connector name).",
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
			color.Yellow("[dry-run] Would delete connector %s\n", name)
			return
		}

		if !deleteYes {
			var confirmed bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Delete " + name + "?",
				Default: false,
			}, &confirmed); err != nil {
				color.Yellow("Canceled\n")
				return
			}
			if !confirmed {
				color.Yellow("Canceled\n")
				return
			}
		}

		if err := client.DeleteConnector(cmd.Context(), name); err != nil {
			color.Red("Failed to delete connector: %v\n", err)
			return
		}
		color.Green("Connector %s deleted\n", name)
	},
}

func init() {
	DeleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "Skip the confirmation prompt")
}
