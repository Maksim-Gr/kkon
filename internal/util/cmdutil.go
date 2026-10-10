package util

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// ErrCanceled signals that the user canceled an interactive prompt.
// Execute prints "Canceled" once and exits 0.
var ErrCanceled = errors.New("canceled")

// ErrNothingToDo signals that there was no work to perform (e.g. no
// connectors found). The message is printed at the source; Execute exits 0.
var ErrNothingToDo = errors.New("nothing to do")

// IsDryRun reports whether the global --dry-run flag is set.
func IsDryRun(cmd *cobra.Command) bool {
	return rootFlagValue(cmd, "dry-run") == "true"
}

// IsJSONOutput reports whether the global --output flag is set to json.
func IsJSONOutput(cmd *cobra.Command) bool {
	return rootFlagValue(cmd, "output") == "json"
}

func rootFlagValue(cmd *cobra.Command, name string) string {
	f := cmd.Root().PersistentFlags().Lookup(name)
	if f == nil {
		return ""
	}
	return f.Value.String()
}

// IsTerminal reports whether f is attached to a terminal.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
