package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	versionStr = "dev"
	commitStr  = "none"
	dateStr    = "unknown"
)

// SetVersionInfo records build metadata and enables `gk --version`.
// It is called from main before Execute.
func SetVersionInfo(version, commit, date string) {
	versionStr, commitStr, dateStr = version, commit, date
	RootCmd.Version = version
}

// versionCmd prints detailed build metadata.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("gk %s\n", versionStr)
		fmt.Printf("commit: %s\n", commitStr)
		fmt.Printf("built:  %s\n", dateStr)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
