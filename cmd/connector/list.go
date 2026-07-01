package connector

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
		expanded, err := client.ListConnectorsExpanded(cmd.Context())
		stop()

		if err != nil {
			if jsonMode {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			} else {
				color.Red("Failed to list connectors: %v\n", err)
			}
			return
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
			if ex, ok := expanded[name]; ok {
				badge = "  " + util.ColorState(ex.Status.Connector.State)
			}
			fmt.Printf("  %-*s%s\n", maxLen, name, badge)
		}

		selected := listConfigName
		if selected == "" {
			const cancelOpt = "← Cancel"
			prompt := &survey.Select{
				Message: "Select connector:",
				Options: append(connectors, cancelOpt),
			}
			if err := survey.AskOne(prompt, &selected); err != nil || selected == cancelOpt {
				color.Yellow("Canceled\n")
				return
			}

			const showStatusOpt, showConfigOpt, editOpt, cancelAction = "Show status", "Show config", "Edit config", "← Cancel"
			var action string
			if err := survey.AskOne(&survey.Select{
				Message: "Action for " + selected + ":",
				Options: []string{showStatusOpt, showConfigOpt, editOpt, cancelAction},
			}, &action); err != nil || action == cancelAction {
				color.Yellow("Canceled\n")
				return
			}

			switch action {
			case showStatusOpt:
				color.Cyan("Status for %s:\n", selected)
				printConnectorStatus(cmd.Context(), client, selected, expanded[selected].Status)
				return
			case editOpt:
				if err := editConnectorConfig(cmd.Context(), client, selected); err != nil {
					color.Red("%v\n", err)
				}
				return
			}
			// showConfigOpt falls through to config display below.
		}

		info := expanded[selected].Info
		color.Green("config for %s connector:\n", selected)
		pretty, err := util.ToPrettyJSON(info.Config)
		if err != nil {
			b, _ := json.MarshalIndent(info.Config, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Println(pretty)
	},
}

func init() {
	ListCmd.Flags().StringVarP(&listConfigName, "config", "c", "", "Print config for the named connector (skips interactive prompt)")
	ListCmd.Flags().StringVar(&listState, "state", "", "Only show connectors in this state (e.g. RUNNING, FAILED, PAUSED)")
}

// filterByStateExpanded returns names whose connector state matches want (case-insensitive).
func filterByStateExpanded(names []string, expanded map[string]connector.ConnectorExpanded, want string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if ex, ok := expanded[name]; ok && strings.EqualFold(ex.Status.Connector.State, want) {
			out = append(out, name)
		}
	}
	return out
}
