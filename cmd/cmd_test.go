package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const (
	defaultBuildVersion = "dev"
	defaultBuildCommit  = "unknown"
)

// newTestCommand creates a fresh root command with version subcommand
// for isolated testing.
func newTestCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "ts-bridge",
		Short: "Portable TCP bridge over Tailscale mesh networks",
	}
	root.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")
	root.PersistentFlags().String("config", "", "Config file path (reserved for future use)")

	version := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
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
	version.Flags().Bool("short", false, "Print just the semver version")
	root.AddCommand(version)

	return root
}

func TestRootHelp(t *testing.T) {
	BuildVersion = defaultBuildVersion
	BuildCommit = defaultBuildCommit

	cmd := newTestCommand()
	cmd.SetArgs([]string{"--help"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "version") {
		t.Errorf("help output should mention 'version' subcommand, got:\n%s", output)
	}
	if !strings.Contains(output, "ts-bridge") {
		t.Errorf("help output should mention 'ts-bridge', got:\n%s", output)
	}
}

func TestVersionCommandDefault(t *testing.T) {
	BuildVersion = defaultBuildVersion
	BuildCommit = defaultBuildCommit

	cmd := newTestCommand()
	cmd.SetArgs([]string{"version"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "ts-bridge") {
		t.Errorf("output should contain 'ts-bridge', got:\n%s", output)
	}
	if !strings.Contains(output, "commit") {
		t.Errorf("output should contain 'commit', got:\n%s", output)
	}
}

func TestVersionCommandShort(t *testing.T) {
	BuildVersion = defaultBuildVersion
	BuildCommit = defaultBuildCommit

	cmd := newTestCommand()
	cmd.SetArgs([]string{"version", "--short"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := strings.TrimSpace(buf.String())
	if output != defaultBuildVersion {
		t.Errorf("expected %q, got %q", defaultBuildVersion, output)
	}
}

func TestVersionWithBuildVars(t *testing.T) {
	BuildVersion = "1.0.0"
	BuildCommit = "abc1234"

	cmd := newTestCommand()
	cmd.SetArgs([]string{"version"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "1.0.0") {
		t.Errorf("output should contain version '1.0.0', got:\n%s", output)
	}
	if !strings.Contains(output, "abc1234") {
		t.Errorf("output should contain commit 'abc1234', got:\n%s", output)
	}
}

func TestVersionShortWithBuildVars(t *testing.T) {
	BuildVersion = "2.1.0"
	BuildCommit = "deadbeef"

	cmd := newTestCommand()
	cmd.SetArgs([]string{"version", "--short"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := strings.TrimSpace(buf.String())
	if output != "2.1.0" {
		t.Errorf("expected '2.1.0', got %q", output)
	}
}
