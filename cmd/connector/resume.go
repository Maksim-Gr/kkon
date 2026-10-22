package connector

import (
	"context"

	"github.com/Maksim-Gr/kkon/cmd/action"
	"github.com/Maksim-Gr/kkon/internal/connector"

	"github.com/spf13/cobra"
)

// ResumeCmd resumes one or more connectors.
var ResumeCmd = &cobra.Command{
	Use:   "resume [name...]",
	Short: "Resume connectors",
	Long: `Resume one or more paused connectors and their tasks. Run without arguments to
multi-select interactively, pass connector names, or use --all. Confirmation
is asked only when targeting more than one connector (skip it with --yes).`,
	Example: `  # Pick connectors to resume interactively
  kkon connector resume

  # Resume a specific connector
  kkon connector resume my-connector

  # Resume all connectors without confirmation
  kkon connector resume --all --yes`,
	Args: cobra.ArbitraryArgs,
	RunE: lifecycleRunE(action.Spec{
		Verb:           "resume",
		SuccessFmt:     "Resume requested for %s\n",
		ConfirmDefault: true,
	}, func(ctx context.Context, client *connector.Client, name string) error {
		return client.ResumeConnector(ctx, name)
	}),
}

func init() {
	ResumeCmd.Flags().Bool("all", false, "Resume all connectors")
	ResumeCmd.Flags().BoolP("yes", "y", false, "Skip the confirmation prompt for multiple connectors")
}
