package connector

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Maksim-Gr/kkon/internal/connector"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
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
	Short: "List running connectors",
	Long:  `List current running connector`,
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

		stop := util.StartSpinner("Fetching connectors...")
		connectors, err := client.ListConnectors(cmd.Context())
		statuses, _ := client.ListConnectorStatuses(cmd.Context())
		stop()

		if err != nil {
			if jsonMode {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			} else {
				color.Red("Failed to list connector: %v\n", err)
			}
			return
		}

		if listState != "" {
			connectors = filterByState(connectors, statuses, listState)
		}

		if jsonMode {
			type entry struct {
				Name  string `json:"name"`
				State string `json:"state,omitempty"`
			}
			out := make([]entry, 0, len(connectors))
			for _, name := range connectors {
				e := entry{Name: name}
				if s, ok := statuses[name]; ok {
					e.State = s.Connector.State
				}
				out = append(out, e)
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(b))
			return
		}

		if len(connectors) == 0 {
			color.Yellow("No connectors found\n")
			return
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
			if s, ok := statuses[name]; ok {
				badge = "  " + util.ColorState(s.Connector.State)
			}
			fmt.Printf("  %-*s%s\n", maxLen, name, badge)
		}

		selected := listConfigName
		if selected == "" {
			const cancelOpt = "← Cancel"
			prompt := &survey.Select{
				Message: "Show connector config:",
				Options: append(connectors, cancelOpt),
			}
			if err := survey.AskOne(prompt, &selected); err != nil || selected == cancelOpt {
				color.Yellow("Canceled\n")
				return
			}
		}

		config, err := client.GetConnectorConfig(cmd.Context(), selected)
		if err != nil {
			color.Red("Failed to get connector config: %v\n", err)
			return
		}
		color.Green("config for %s connector:\n", selected)
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(config), &raw); err != nil {
			fmt.Println(config)
			return
		}
		pretty, err := util.ToPrettyJSON(raw)
		if err != nil {
			fmt.Println(config)
			return
		}
		fmt.Println(pretty)
	},
}

func init() {
	ListCmd.Flags().StringVarP(&listConfigName, "config", "c", "", "Print config for the named connector (skips interactive prompt)")
	ListCmd.Flags().StringVar(&listState, "state", "", "Only show connectors in this state (e.g. RUNNING, FAILED, PAUSED)")
}

// filterByState returns the names whose connector state matches want (case-insensitive).
// Connectors without a known status are excluded.
func filterByState(names []string, statuses connector.ConnectorsStatusResponse, want string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if s, ok := statuses[name]; ok && strings.EqualFold(s.Connector.State, want) {
			out = append(out, name)
		}
	}
	return out
}
