package connector

import (
	"github.com/Maksim-Gr/kkon/internal/connector"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// UpdateCmd interactively updates an existing connector's configuration.
var UpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an existing connector configuration",
	Long:  `Fetch a connector's live config and edit fields interactively, then apply the changes.`,
	Run: func(cmd *cobra.Command, _ []string) {
		cfg, err := util.LoadConfig()
		if err != nil {
			color.Red("Failed to load config: %v\n", err)
			return
		}
		client := connector.NewClient(cfg.KafkaConnect.URL)
		if cfg.KafkaConnect.Username != "" {
			client.SetBasicAuth(cfg.KafkaConnect.Username, cfg.KafkaConnect.Password)
		}

		connectors, err := client.ListConnectors(cmd.Context())
		if err != nil {
			color.Red("Failed to list connectors: %v\n", err)
			return
		}
		if len(connectors) == 0 {
			color.Yellow("No connectors found\n")
			return
		}

		var selected string
		if err := survey.AskOne(&survey.Select{
			Message: "Select connector to update:",
			Options: connectors,
		}, &selected); err != nil {
			color.Yellow("Canceled\n")
			return
		}

		if err := editConnectorConfig(cmd.Context(), client, selected); err != nil {
			color.Red("%v\n", err)
		}
	},
}
