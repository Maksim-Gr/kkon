package connector

import (
	"gokafkaconnect/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// ResumeCmd resumes a connector.
var ResumeCmd = &cobra.Command{
	Use:   "resume [name]",
	Short: "Resume a connector",
	Long:  "Resumes a paused Kafka Connect connector and its tasks (select interactively or pass the connector name).",
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
			color.Yellow("[dry-run] Would resume connector %s\n", name)
			return
		}

		if err := client.ResumeConnector(cmd.Context(), name); err != nil {
			color.Red("Failed to resume %s: %v\n", name, err)
			return
		}
		color.Green("Resume requested for %s\n", name)
	},
}
