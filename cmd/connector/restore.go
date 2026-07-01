package connector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Maksim-Gr/kkon/internal/connector"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	restoreDir string
	restoreYes bool
)

// RestoreCmd re-creates connectors from a backup JSON file.
var RestoreCmd = &cobra.Command{
	Use:   "restore [file]",
	Short: "Restore connectors from a backup file",
	Long:  "Re-create connectors from a backup JSON file produced by 'kkon connector backup'.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file, ok := resolveBackupFile(argOrEmpty(args), restoreDir)
		if !ok {
			return
		}

		configs, err := loadBackupFile(file)
		if err != nil {
			color.Red("%v\n", err)
			return
		}
		if len(configs) == 0 {
			color.Yellow("No connectors found in %s\n", file)
			return
		}

		client, ok := util.NewKafkaConnectClient()
		if !ok {
			return
		}

		if isDryRun(cmd) {
			color.Yellow("[dry-run] Would restore %d connector(s) from %s:\n", len(configs), file)
			for _, name := range sortedKeys(configs) {
				color.Yellow("  - %s\n", name)
			}
			return
		}

		// Detect connectors that already exist so we can confirm overwrites.
		existing := map[string]bool{}
		if names, err := client.ListConnectors(cmd.Context()); err == nil {
			for _, n := range names {
				existing[n] = true
			}
		}

		toRestore := make(map[string]map[string]string, len(configs))
		for _, name := range sortedKeys(configs) {
			if existing[name] && !restoreYes {
				var overwrite bool
				if err := survey.AskOne(&survey.Confirm{
					Message: "Connector " + name + " already exists. Overwrite?",
					Default: false,
				}, &overwrite); err != nil || !overwrite {
					color.Yellow("Skipping %s\n", name)
					continue
				}
			}
			toRestore[name] = configs[name]
		}

		if len(toRestore) == 0 {
			color.Yellow("Nothing to restore\n")
			return
		}

		stop := util.StartSpinner("Restoring connectors...")
		restored, err := connector.RestoreConnectorConfigs(cmd.Context(), client, toRestore)
		stop()
		if err != nil {
			color.Red("Failed after restoring %d connector(s): %v\n", len(restored), err)
			return
		}
		color.Green("Successfully restored %d connector(s)\n", len(restored))
	},
}

func init() {
	RestoreCmd.Flags().StringVar(&restoreDir, "dir", "./backup", "Directory to look for backup files when no file is given")
	RestoreCmd.Flags().BoolVarP(&restoreYes, "yes", "y", false, "Skip overwrite confirmation for existing connectors")
}

// resolveBackupFile returns the backup file to use: the provided path, or an
// interactive pick from the newest *.json files in dir.
func resolveBackupFile(file, dir string) (string, bool) {
	if file != "" {
		return file, true
	}

	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(entries) == 0 {
		color.Red("No backup files found in %s (pass a file path explicitly)\n", dir)
		return "", false
	}
	// Timestamped names sort chronologically; reverse for newest first.
	sort.Sort(sort.Reverse(sort.StringSlice(entries)))

	const cancelOpt = "← Cancel"
	var selected string
	prompt := &survey.Select{Message: "Pick a backup file:", Options: append(entries, cancelOpt)}
	if err := survey.AskOne(prompt, &selected); err != nil || selected == cancelOpt {
		color.Yellow("Canceled\n")
		return "", false
	}
	return selected, true
}

// loadBackupFile reads and parses a backup file into name→config maps.
func loadBackupFile(path string) (map[string]map[string]string, error) {
	b, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var configs map[string]map[string]string
	if err := json.Unmarshal(b, &configs); err != nil {
		return nil, fmt.Errorf("invalid backup file %s: %w", path, err)
	}
	return configs, nil
}

// sortedKeys returns the map keys in stable sorted order.
func sortedKeys(m map[string]map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
