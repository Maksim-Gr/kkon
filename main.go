// Package main is the entry point for the kkon CLI tool.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Maksim-Gr/kkon/cmd"
)

// Build metadata, injected via -ldflags at release time (see .goreleaser.yaml).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd.Execute(ctx)
}
