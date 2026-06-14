// Package config provides CLI commands for managing kkon configuration.
package config

import (
	"github.com/spf13/cobra"
)

// Cmd is the root command for configuration management.
var Cmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
	Long:  `Manage Kafka Connect configuration including URL setup and backups.`,
}

func init() {
	Cmd.AddCommand(ConfigureCmd)
	Cmd.AddCommand(ShowConfigCmd)
}
