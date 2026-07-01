package connector

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Maksim-Gr/kkon/internal/connector"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var pluginType string

// PluginsCmd lists the connector plugins installed on the cluster.
var PluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "List connector plugins installed on the cluster",
	Long:  "List the connector plugin classes available on Kafka Connect (useful before create).",
	Run: func(cmd *cobra.Command, _ []string) {
		client, ok := util.NewKafkaConnectClient()
		if !ok {
			return
		}

		jsonMode := cmd.Root().PersistentFlags().Lookup("output").Value.String() == "json"

		stop := util.StartSpinner("Fetching connector plugins...")
		plugins, err := client.ListConnectorPlugins(cmd.Context())
		stop()
		if err != nil {
			if jsonMode {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			} else {
				color.Red("Failed to list connector plugins: %v\n", err)
			}
			return
		}

		if pluginType != "" {
			filtered := make([]connector.Plugin, 0, len(plugins))
			for _, p := range plugins {
				if strings.EqualFold(p.Type, pluginType) {
					filtered = append(filtered, p)
				}
			}
			plugins = filtered
		}

		if jsonMode {
			b, _ := json.MarshalIndent(plugins, "", "  ")
			fmt.Println(string(b))
			return
		}

		if len(plugins) == 0 {
			color.Yellow("No connector plugins found\n")
			return
		}

		maxLen := 0
		for _, p := range plugins {
			if len(p.Class) > maxLen {
				maxLen = len(p.Class)
			}
		}

		color.Cyan("Connector plugins:")
		for _, p := range plugins {
			version := p.Version
			if version == "" {
				version = "unknown"
			}
			fmt.Printf("  %-*s  %-6s  %s\n", maxLen, p.Class, p.Type, version)
		}
	},
}

func init() {
	PluginsCmd.Flags().StringVar(&pluginType, "type", "", "Only show plugins of this type (source or sink)")
}
