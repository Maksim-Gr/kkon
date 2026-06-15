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

var healthState string

// HealthCheckCmd shows connector and task statuses.
var HealthCheckCmd = &cobra.Command{
	Use:   "health-check",
	Short: "Show connector statuses",
	Long:  `Show each connector status`,
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

		jsonMode := cmd.Root().PersistentFlags().Lookup("output").Value.String() == "json"

		stop := util.StartSpinner("Fetching connector statuses...")
		connectorStatuses, err := client.ListConnectorStatuses(cmd.Context())
		stop()
		if err != nil {
			if jsonMode {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			} else {
				color.Red("Failed to list connector statuses: %v\n", err)
			}
			return
		}

		if healthState != "" {
			for name, status := range connectorStatuses {
				if !strings.EqualFold(status.Connector.State, healthState) {
					delete(connectorStatuses, name)
				}
			}
		}

		if jsonMode {
			b, _ := json.MarshalIndent(connectorStatuses, "", "  ")
			fmt.Println(string(b))
			return
		}

		maxLen := 0
		for name := range connectorStatuses {
			if len(name) > maxLen {
				maxLen = len(name)
			}
		}

		color.Cyan("Connector Statuses:")
		for name, status := range connectorStatuses {
			fmt.Printf("  %-*s  %s\n", maxLen, name, util.ColorState(status.Connector.State))
			for _, t := range status.Tasks {
				fmt.Printf("  %-*s    Task %d: %s\n", maxLen, "", t.ID, util.ColorState(t.State))
				if t.State == "FAILED" {
					ts, err := client.GetConnectorTaskStatus(cmd.Context(), name, t.ID)
					if err == nil && ts.Trace != "" {
						lines := strings.Split(strings.TrimRight(ts.Trace, "\n"), "\n")
						shown := lines
						if len(lines) > 3 {
							shown = lines[:3]
						}
						for _, line := range shown {
							color.Yellow("  %-*s      %s\n", maxLen, "", line)
						}
						if len(lines) > 3 {
							color.Yellow("  %-*s      ...\n", maxLen, "")
							fmt.Printf("  %-*s      To see full trace run: kkon task get --connector %s --id %d\n", maxLen, "", name, t.ID)
						}
					}
				}
			}
		}
	},
}

func init() {
	HealthCheckCmd.Flags().StringVar(&healthState, "state", "", "Only show connectors in this state (e.g. RUNNING, FAILED, PAUSED)")
}
