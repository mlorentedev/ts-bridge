// Package main is the entry point for ts-bridge.
package main

import (
	"fmt"
	"os"

	cli "ts-bridge/cmd/cli"
)

// Build-time variables set via ldflags.
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	// Backward compat: the pre-Cobra binary accepted -version/--version as
	// flags. Cobra exposes version via the "version" subcommand instead, so
	// handle the legacy flag forms here before dispatch.
	if cli.LegacyVersionRequested(os.Args) {
		fmt.Printf("ts-bridge %s (commit %s)\n", version, commit)
		os.Exit(0)
	}

	// Normalize legacy flag shorthands (-v -> --verbose) before Cobra parses.
	os.Args = cli.NormalizeArgs(os.Args)

	// Wire build-time variables and the bridge runner into the cmd package.
	cli.BuildVersion = version
	cli.BuildCommit = commit
	cli.Runner = cli.Run
	cli.LoggerInit = cli.InitLogger

	// Let Cobra handle all flag parsing and command dispatch.
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
