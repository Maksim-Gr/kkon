package connector

import (
	"github.com/spf13/cobra"
)

// Cmd is the root command for connector management.
var Cmd = &cobra.Command{
	Use:   "connector",
	Short: "Connector management commands",
	Long:  `Manage Kafka connectors including creation, deletion, listing, and health checks.`,
}

func init() {
	Cmd.AddCommand(CreateCmd)
	Cmd.AddCommand(DeleteCmd)
	Cmd.AddCommand(ListCmd)
	Cmd.AddCommand(UpdateCmd)
	Cmd.AddCommand(HealthCheckCmd)
	Cmd.AddCommand(BackupCmd)
	Cmd.AddCommand(RestoreCmd)
	Cmd.AddCommand(PluginsCmd)
	Cmd.AddCommand(PauseCmd)
	Cmd.AddCommand(ResumeCmd)
	Cmd.AddCommand(RestartCmd)
}
