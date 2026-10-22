package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Maksim-Gr/kkon/cmd/action"
	"github.com/Maksim-Gr/kkon/internal/prompt"
	"github.com/Maksim-Gr/kkon/internal/util"

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
	Long: `Re-create connectors from a backup JSON file produced by 'kkon connector
backup'. Without a file argument, pick one interactively from --dir. Existing
connectors are only overwritten after confirmation (skip it with --yes).`,
	Example: `  # Pick a backup file from ./backup interactively
  kkon connector restore

  # Restore a specific backup file
  kkon connector restore ./backup/config_20260710_120000.json

  # Overwrite existing connectors without confirmation
  kkon connector restore ./backup/config_20260710_120000.json --yes

  # Preview without applying
  kkon connector restore ./backup/config_20260710_120000.json --dry-run`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if util.IsJSONOutput(cmd) && (argOrEmpty(args) == "" || !restoreYes) {
			return action.ErrNonInteractiveJSON
		}

		file, err := resolveBackupFile(argOrEmpty(args), restoreDir)
		if err != nil {
			return err
		}

		configs, err := loadBackupFile(file)
		if err != nil {
			return err
		}
		if len(configs) == 0 {
			color.Yellow("No connectors found in %s\n", file)
			return util.ErrNothingToDo
		}

		client, err := util.NewKafkaConnectClient()
		if err != nil {
			return err
		}

		// Detect connectors that already exist so we can confirm overwrites.
		existing := map[string]bool{}
		if !util.IsDryRun(cmd) && !restoreYes {
			if names, err := client.ListConnectors(cmd.Context()); err == nil {
				for _, n := range names {
					existing[n] = true
				}
			}
		}

		// PUT /connectors/{name}/config upserts, so restore both re-creates
		// missing connectors and overwrites existing ones.
		return action.Run(cmd, action.Spec{
			Verb:       "restore",
			Batch:      true,
			DryRunNote: func() string { return " from " + file },
			ConfirmEach: func(t action.Target) (bool, error) {
				if !existing[t.Connector] {
					return true, nil
				}
				return prompt.Confirm("Connector "+t.Connector+" already exists. Overwrite?", false)
			},
			Op: func(ctx context.Context, t action.Target) error {
				return client.UpdateConnectorConfig(ctx, t.Connector, configs[t.Connector])
			},
		}, action.Connectors(sortedKeys(configs)))
	},
}

func init() {
	RestoreCmd.Flags().StringVar(&restoreDir, "dir", "./backup", "Directory to look for backup files when no file is given")
	RestoreCmd.Flags().BoolVarP(&restoreYes, "yes", "y", false, "Skip overwrite confirmation for existing connectors")
}

// resolveBackupFile returns the backup file to use: the provided path, or an
// interactive pick from the newest *.json files in dir.
func resolveBackupFile(file, dir string) (string, error) {
	if file != "" {
		return file, nil
	}

	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("no backup files found in %s (pass a file path explicitly)", dir)
	}
	// Timestamped names sort chronologically; reverse for newest first.
	sort.Sort(sort.Reverse(sort.StringSlice(entries)))

	const cancelOpt = "← Cancel"
	i, err := prompt.Select("Pick a backup file:", append(entries, cancelOpt), prompt.Options{})
	if err != nil {
		return "", err
	}
	if i == len(entries) {
		return "", util.ErrCanceled
	}
	return entries[i], nil
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
