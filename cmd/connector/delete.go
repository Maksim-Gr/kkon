package connector

import (
	"context"

	"github.com/Maksim-Gr/kkon/cmd/action"
	"github.com/Maksim-Gr/kkon/internal/connector"

	"github.com/spf13/cobra"
)

// DeleteCmd represents the delete command.
var DeleteCmd = &cobra.Command{
	Use:   "delete [name...]",
	Short: "Delete connectors",
	Long: `Delete one or more connectors from Kafka Connect. Run without arguments to
multi-select interactively, pass connector names, or use --all. Deletion asks
for confirmation unless --yes is given.`,
	Example: `  # Pick connectors to delete interactively
  kkon connector delete

  # Delete a specific connector
  kkon connector delete my-connector

  # Delete several connectors without confirmation
  kkon connector delete connector-a connector-b --yes

  # Delete every connector (scripts/CI)
  kkon connector delete --all --yes

  # Preview deletions without executing them
  kkon connector delete --all --dry-run`,
	Args: cobra.ArbitraryArgs,
	RunE: lifecycleRunE(action.Spec{
		Verb:          "delete",
		SuccessFmt:    "Connector %s deleted\n",
		ConfirmSingle: true,
	}, func(ctx context.Context, client *connector.Client, name string) error {
		return client.DeleteConnector(ctx, name)
	}),
}

func init() {
	DeleteCmd.Flags().Bool("all", false, "Delete all connectors")
	DeleteCmd.Flags().BoolP("yes", "y", false, "Skip the confirmation prompt")
}
