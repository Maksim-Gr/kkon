package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// ShowConfigCmd represents the showConfig command.
var ShowConfigCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long: `Print the config file path and the effective configuration (file values
overridden by KKON_* environment variables). The password is masked.`,
	Example: `  kkon config show`,
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := util.ResolveConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if cfg.KafkaConnect.Password != "" {
			cfg.KafkaConnect.Password = "********"
		}

		if configPath, err := util.GetConfigPath(); err == nil {
			color.Cyan("Config file: %s\n", configPath)
		}
		if env := util.EnvOverrides(); len(env) > 0 {
			color.Yellow("Overridden by env: %s\n", strings.Join(env, ", "))
		}

		color.Cyan("Current Configuration:")
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format config: %w", err)
		}
		fmt.Printf("\n%s\n\n", string(data))
		return nil
	},
}
