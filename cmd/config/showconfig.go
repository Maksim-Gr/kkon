package config

import (
	"encoding/json"
	"fmt"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// ShowConfigCmd represents the showConfig command.
var ShowConfigCmd = &cobra.Command{
	Use:   "show",
	Short: "Display API endpoint",
	Long:  `Display Kafka Connect API endpoint.`,
	Run: func(_ *cobra.Command, _ []string) {
		cfg, err := util.LoadConfig()
		if err != nil {
			color.Red("Failed to load config: %v\n", err)
			return
		}

		if cfg.KafkaConnect.Password != "" {
			cfg.KafkaConnect.Password = "********"
		}

		if configPath, err := util.GetConfigPath(); err == nil {
			color.Cyan("Config file: %s\n", configPath)
		}

		color.Cyan("Current Configuration:")
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			color.Red("Failed to format config: %v\n", err)
			return
		}
		fmt.Printf("\n%s\n\n", string(data))
	},
}
