package connector

import (
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// PauseCmd pauses a connector.
var PauseCmd = &cobra.Command{
	Use:   "pause [name]",
	Short: "Pause a connector",
	Long:  "Pauses a Kafka Connect connector and its tasks (select interactively or pass the connector name).",
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
			color.Yellow("[dry-run] Would pause connector %s\n", name)
			return
		}

		if err := client.PauseConnector(cmd.Context(), name); err != nil {
			color.Red("Failed to pause %s: %v\n", name, err)
			return
		}
		color.Green("Pause requested for %s\n", name)
	},
}
