package connector

import (
	"context"

	"github.com/Maksim-Gr/kkon/cmd/action"
	"github.com/Maksim-Gr/kkon/internal/connector"

	"github.com/spf13/cobra"
)

// PauseCmd pauses one or more connectors.
var PauseCmd = &cobra.Command{
	Use:   "pause [name...]",
	Short: "Pause connectors",
	Long: `Pause one or more connectors and their tasks. Run without arguments to
multi-select interactively, pass connector names, or use --all. Confirmation
is asked only when targeting more than one connector (skip it with --yes).`,
	Example: `  # Pick connectors to pause interactively
  kkon connector pause

  # Pause a specific connector
  kkon connector pause my-connector

  # Pause all connectors without confirmation
  kkon connector pause --all --yes

  # Preview without pausing anything
  kkon connector pause --all --dry-run`,
	Args: cobra.ArbitraryArgs,
	RunE: lifecycleRunE(action.Spec{
		Verb:           "pause",
		SuccessFmt:     "Pause requested for %s\n",
		ConfirmDefault: true,
	}, func(ctx context.Context, client *connector.Client, name string) error {
		return client.PauseConnector(ctx, name)
	}),
}

func init() {
	PauseCmd.Flags().Bool("all", false, "Pause all connectors")
	PauseCmd.Flags().BoolP("yes", "y", false, "Skip the confirmation prompt for multiple connectors")
}
