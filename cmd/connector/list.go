package connector

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Maksim-Gr/kkon/internal/connector"
	"github.com/Maksim-Gr/kkon/internal/prompt"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	listConfigName string
	listState      string
)

// ListCmd represent command for retrieving connectors from API.
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List connectors and inspect their configs",
	Long: `List all connectors with their state, then optionally pick one to print its
configuration. Use --config to print a named connector's config directly,
--state to filter, or --output json for machine-readable output.`,
	Example: `  # List connectors and pick one to inspect
  kkon connector list

  # Print the config of a specific connector
  kkon connector list --config my-connector

  # Show only failed connectors
  kkon connector list --state FAILED

  # Machine-readable output
  kkon connector list --output json`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := util.NewKafkaConnectClient()
		if err != nil {
			return err
		}

		jsonMode := util.IsJSONOutput(cmd)

		stop := util.StartSpinner("Fetching connectors...")
		expanded, err := client.ListConnectorsExpanded(cmd.Context())
		stop()
		if err != nil {
			return fmt.Errorf("failed to list connectors: %w", err)
		}

		// Non-interactive path: --config flag provided.
		if listConfigName != "" {
			return printConnectorConfig(listConfigName, expanded, jsonMode)
		}

		// Build a sorted slice of names.
		connectors := make([]string, 0, len(expanded))
		for name := range expanded {
			connectors = append(connectors, name)
		}
		sort.Strings(connectors)

		if listState != "" {
			connectors = filterByStateExpanded(connectors, expanded, listState)
		}

		if jsonMode {
			type entry struct {
				Name  string `json:"name"`
				State string `json:"state,omitempty"`
			}
			out := make([]entry, 0, len(connectors))
			for _, name := range connectors {
				e := entry{Name: name}
				if ex, ok := expanded[name]; ok {
					e.State = ex.Status.Connector.State
				}
				out = append(out, e)
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(b))
			return nil
		}

		if len(connectors) == 0 {
			color.Yellow("No connectors found\n")
			return nil
		}

		maxLen := 0
		for _, name := range connectors {
			if len(name) > maxLen {
				maxLen = len(name)
			}
		}

		color.Cyan("Connectors:")
		for _, name := range connectors {
			badge := ""
			if ex, ok := expanded[name]; ok {
				badge = "  " + util.ColorState(ex.Status.Connector.State)
			}
			fmt.Printf("  %-*s%s\n", maxLen, name, badge)
		}

		const cancelOpt = "← Cancel"
		i, err := prompt.Select("Select connector:", append(connectors, cancelOpt), prompt.Options{})
		if err != nil {
			return err
		}
		if i == len(connectors) {
			return util.ErrCanceled
		}
		selected := connectors[i]

		const showConfigOpt, editOpt, cancelAction = "Show config", "Edit config", "← Cancel"
		actions := []string{showConfigOpt, editOpt, cancelAction}
		j, err := prompt.Select("Action for "+selected+":", actions, prompt.Options{})
		if err != nil {
			return err
		}
		action := actions[j]
		if action == cancelAction {
			return util.ErrCanceled
		}

		if action == editOpt {
			return editConnectorConfig(cmd.Context(), client, selected, util.IsDryRun(cmd))
		}

		return printConnectorConfig(selected, expanded, jsonMode)
	},
}

func init() {
	ListCmd.Flags().StringVarP(&listConfigName, "config", "c", "", "Print config for the named connector (skips interactive prompt)")
	ListCmd.Flags().StringVar(&listState, "state", "", "Only show connectors in this state (e.g. RUNNING, FAILED, PAUSED)")
}

// printConnectorConfig prints the named connector's config as indented JSON.
// In json mode only the raw config is written to stdout.
func printConnectorConfig(name string, expanded map[string]connector.ExpandedEntry, jsonMode bool) error {
	entry, ok := expanded[name]
	if !ok {
		return fmt.Errorf("connector %s not found", name)
	}
	pretty, err := util.ToPrettyJSON(entry.Info.Config)
	if err != nil {
		return fmt.Errorf("failed to format config: %w", err)
	}
	if !jsonMode {
		color.Green("config for %s connector:\n", name)
	}
	fmt.Println(pretty)
	return nil
}

// filterByStateExpanded returns names whose connector state matches want (case-insensitive).
func filterByStateExpanded(names []string, expanded map[string]connector.ExpandedEntry, want string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if ex, ok := expanded[name]; ok && strings.EqualFold(ex.Status.Connector.State, want) {
			out = append(out, name)
		}
	}
	return out
}
