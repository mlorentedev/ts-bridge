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
	// Backward compat: handle deprecated -version and -v flags.
	for i, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" {
			fmt.Printf("ts-bridge %s (commit %s)\n", version, commit)
			os.Exit(0)
		}
		if arg == "-v" {
			newArgs := make([]string, 0, len(os.Args))
			newArgs = append(newArgs, os.Args[:i+1]...)
			newArgs[i] = "--verbose"
			newArgs = append(newArgs, os.Args[i+2:]...)
			os.Args = newArgs
			break
		}
	}

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
