// Package cmd contains the root command and CLI entry point.
package cmd

import (
	"gokafkaconnect/cmd/config"
	"gokafkaconnect/cmd/connector"
	"gokafkaconnect/cmd/task"
	"os"

	"github.com/fatih/color"

	"gokafkaconnect/internal/util"

	"github.com/spf13/cobra"
)

// DryRun is the global dry-run flag shared across subcommands.
var DryRun bool

// OutputFormat is the global output format flag ("text" or "json").
var OutputFormat string

// RootCmd represents the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:   "gk",
	Short: "CLI to manage Kafka connector fast and easy!",
	Long: `gk - cli tool for working with Kafka Connect.
	Manage, create, and list predefined connector in seconds!`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Pass dryRun flag to subpackages
		config.SetDryRun(DryRun)

		// Skip config check for config subcommands (configure, show-config, backup)
		// so the user isn't double-prompted on first-time setup.
		if cmd.Parent() != nil && cmd.Parent().Use == "config" {
			return
		}

		cfg, err := util.LoadConfig()
		if err != nil || cfg.KafkaConnect.URL == "" {
			color.Yellow("No Kafka Connect URL configured.")
			color.Cyan("Running initial configuration...\n")
			if err := config.ConfigureCmd.RunE(cmd, args); err != nil {
				color.Red("Configuration failed: %v\n", err)
				os.Exit(1)
			}
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	RootCmd.PersistentFlags().BoolVarP(&DryRun, "dry-run", "d", false, "Dry run mode")
	RootCmd.PersistentFlags().StringVarP(&OutputFormat, "output", "o", "text", "Output format: text or json")

	// Bind global flags to subpackages
	task.BindGlobals(&DryRun)

	// Set up command tree
	RootCmd.AddCommand(task.Cmd)
	RootCmd.AddCommand(config.Cmd)
	RootCmd.AddCommand(connector.Cmd)
}
