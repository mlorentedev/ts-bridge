package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd is the "ts-bridge version" subcommand.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  `Print the version information of ts-bridge including the version number, git commit, and Go version.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		short, _ := cmd.Flags().GetBool("short")
		if short {
			fmt.Fprintln(cmd.OutOrStdout(), Version())
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "ts-bridge %s (commit %s)\n", Version(), Commit())
		return nil
	},
}

func init() {
	versionCmd.Flags().Bool("short", false, "Print just the semver version")
	rootCmd.AddCommand(versionCmd)
}
