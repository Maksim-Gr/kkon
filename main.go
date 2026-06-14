// Package main is the entry point for the gk CLI tool.
package main

import "github.com/Maksim-Gr/kkon/cmd"

// Build metadata, injected via -ldflags at release time (see .goreleaser.yaml).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}
