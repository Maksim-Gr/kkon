package connector

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	healthState         string
	healthFailUnhealthy bool
)

// HealthCheckCmd shows connector and task statuses.
var HealthCheckCmd = &cobra.Command{
	Use:   "health-check",
	Short: "Show the status of every connector",
	Long: `Print a status summary for each connector and its tasks. Use --state to filter
and --output json for machine-readable output. With --fail-on-unhealthy the
command exits 1 when any listed connector or task is not RUNNING.`,
	Example: `  # Show all connector statuses
  kkon connector health-check

  # Show only failed connectors
  kkon connector health-check --state FAILED

  # Machine-readable output
  kkon connector health-check --output json

  # Exit non-zero if any connector or task is not RUNNING (for CI/monitoring)
  kkon connector health-check --fail-on-unhealthy`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := util.NewKafkaConnectClient()
		if err != nil {
			return err
		}

		stop := util.StartSpinner("Fetching connector statuses...")
		connectorStatuses, err := client.ListConnectorStatuses(cmd.Context())
		stop()
		if err != nil {
			return fmt.Errorf("failed to list connector statuses: %w", err)
		}

		if healthState != "" {
			for name, status := range connectorStatuses {
				if !strings.EqualFold(status.Connector.State, healthState) {
					delete(connectorStatuses, name)
				}
			}
		}

		if util.IsJSONOutput(cmd) {
			b, _ := json.MarshalIndent(connectorStatuses, "", "  ")
			fmt.Println(string(b))
		} else {
			color.Cyan("Connector Statuses:")
			for _, name := range slices.Sorted(maps.Keys(connectorStatuses)) {
				printConnectorStatus(cmd.Context(), client, name, connectorStatuses[name])
			}
		}

		if healthFailUnhealthy {
			unhealthy := 0
			for _, status := range connectorStatuses {
				if !connectorHealthy(status) {
					unhealthy++
				}
			}
			if unhealthy > 0 {
				return fmt.Errorf("%d of %d connector(s) unhealthy", unhealthy, len(connectorStatuses))
			}
		}
		return nil
	},
}

func init() {
	HealthCheckCmd.Flags().StringVar(&healthState, "state", "", "Only show connectors in this state (e.g. RUNNING, FAILED, PAUSED)")
	HealthCheckCmd.Flags().BoolVar(&healthFailUnhealthy, "fail-on-unhealthy", false, "Exit 1 if any listed connector or task is not RUNNING")
}
