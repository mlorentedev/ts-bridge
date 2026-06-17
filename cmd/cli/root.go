// Package cmd provides the Cobra CLI for ts-bridge.
package cmd

import (
	"github.com/spf13/cobra"
)

// rootCmd is the root Cobra command.
var rootCmd = &cobra.Command{
	Use:   "ts-bridge",
	Short: "Portable TCP bridge over Tailscale mesh networks",
	Long: `ts-bridge is a portable TCP bridge that runs over Tailscale/Headscale
mesh networks using tsnet. It forwards local TCP connections to a remote
target host through the encrypted Tailscale network.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Global flags resolved here so subcommands can access them early.
	},
	// We intentionally do NOT set Run or RunE here; the binary
	// delegates to a subcommand (or exits with help).
}

// Execute adds all child commands to the root command and sets flags
// appropriately. It is called after main() and executes the desired subcommand.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags available on every subcommand.
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().String("config", "", "Config file path (reserved for future use)")
}

// BuildVersion is set by main() at startup from the build-time ldflags variable.
var BuildVersion = "dev"

// BuildCommit is set by main() at startup from the build-time ldflags variable.
var BuildCommit = "unknown"

// Version returns the version string injected at build time.
func Version() string {
	return BuildVersion
}

// Commit returns the commit hash injected at build time.
func Commit() string {
	return BuildCommit
}
