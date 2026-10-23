package connector

import (
	"context"
	"fmt"

	"github.com/Maksim-Gr/kkon/cmd/action"
	"github.com/Maksim-Gr/kkon/internal/connector"

	"github.com/spf13/cobra"
)

var (
	restartIncludeTasks bool
	restartOnlyFailed   bool
)

// RestartCmd restarts one or more connectors.
var RestartCmd = &cobra.Command{
	Use:   "restart [name...]",
	Short: "Restart connectors",
	Long: `Restart one or more connectors, including their tasks by default
(--include-tasks). Run without arguments to multi-select interactively, pass
connector names, or use --all. Use --only-failed to restart only FAILED
connectors and tasks; skip confirmation with --yes.`,
	Example: `  # Pick connectors to restart interactively
  kkon connector restart

  # Restart a connector and its tasks
  kkon connector restart my-connector

  # Restart only what has FAILED
  kkon connector restart my-connector --only-failed

  # Restart several connectors without confirmation
  kkon connector restart connector-a connector-b --yes

  # Preview without restarting anything
  kkon connector restart --all --dry-run`,
	Args: cobra.ArbitraryArgs,
	RunE: lifecycleRunE(action.Spec{
		Verb:           "restart",
		SuccessFmt:     "Restart requested for %s\n",
		ConfirmSingle:  true,
		ConfirmDefault: true,
		DryRunNote: func() string {
			return fmt.Sprintf(" (includeTasks=%t, onlyFailed=%t)", restartIncludeTasks, restartOnlyFailed)
		},
	}, func(ctx context.Context, client *connector.Client, name string) error {
		return client.RestartConnector(ctx, name, restartIncludeTasks, restartOnlyFailed)
	}),
}

func init() {
	RestartCmd.Flags().BoolVar(&restartIncludeTasks, "include-tasks", true, "Also restart the connector's tasks")
	RestartCmd.Flags().BoolVar(&restartOnlyFailed, "only-failed", false, "Restart only FAILED connector and tasks")
	RestartCmd.Flags().Bool("all", false, "Restart all connectors")
	RestartCmd.Flags().BoolP("yes", "y", false, "Skip the confirmation prompt")
}
