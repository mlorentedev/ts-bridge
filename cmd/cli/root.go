// Package cmd provides the Cobra CLI for ts-bridge.
package cmd

import (
	"github.com/spf13/cobra"
)

// Build-time variables set via ldflags.
var (
	//nolint:unused // wired into Runner/BuildCommit in main.go at runtime
	version = "dev"
	//nolint:unused // wired into BuildCommit in main.go at runtime
	commit  = "unknown"
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

// configureUsageOnErrors makes runtime (RunE) failures print just the error,
// not the full usage/help block (#210). Usage is only useful when the user got
// the invocation wrong, so it is still shown for flag-parsing errors via a
// custom FlagErrorFunc. SilenceErrors is set because main() prints the returned
// error itself — without it cobra would print the error a second time.
// Setting these on the root is sufficient: cobra gates usage/error output on
// the root command's flags for every subcommand.
func configureUsageOnErrors(cmd *cobra.Command) {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		_ = c.Usage() // flag-parse error: usage is genuinely helpful here
		return err
	})
}

func init() {
	// Global flags available on every subcommand.
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().String("config", "", "Config file path (reserved for future use)")

	// Runtime errors should not bury themselves under the usage block (#210).
	configureUsageOnErrors(rootCmd)
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
