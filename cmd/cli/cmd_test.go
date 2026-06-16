package cmd_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	cmdpkg "ts-bridge/cmd/cli"
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
		RunE: func(c *cobra.Command, args []string) error {
			short, _ := c.Flags().GetBool("short")
			if short {
				fmt.Fprintln(c.OutOrStdout(), cmdpkg.Version())
				return nil
			}
			fmt.Fprintf(c.OutOrStdout(), "ts-bridge %s (commit %s)\n", cmdpkg.Version(), cmdpkg.Commit())
			return nil
		},
	}
	version.Flags().Bool("short", false, "Print just the semver version")
	root.AddCommand(version)

	return root
}

func TestRootHelp(t *testing.T) {
	cmdpkg.BuildVersion = defaultBuildVersion
	cmdpkg.BuildCommit = defaultBuildCommit

	c := newTestCommand()
	c.SetArgs([]string{"--help"})
	var buf bytes.Buffer
	c.SetOut(&buf)
	if err := c.Execute(); err != nil {
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
	cmdpkg.BuildVersion = defaultBuildVersion
	cmdpkg.BuildCommit = defaultBuildCommit

	c := newTestCommand()
	c.SetArgs([]string{"version"})
	var buf bytes.Buffer
	c.SetOut(&buf)
	if err := c.Execute(); err != nil {
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
	cmdpkg.BuildVersion = defaultBuildVersion
	cmdpkg.BuildCommit = defaultBuildCommit

	c := newTestCommand()
	c.SetArgs([]string{"version", "--short"})
	var buf bytes.Buffer
	c.SetOut(&buf)
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := strings.TrimSpace(buf.String())
	if output != defaultBuildVersion {
		t.Errorf("expected %q, got %q", defaultBuildVersion, output)
	}
}

func TestVersionWithBuildVars(t *testing.T) {
	cmdpkg.BuildVersion = "1.0.0"
	cmdpkg.BuildCommit = "abc1234"

	c := newTestCommand()
	c.SetArgs([]string{"version"})
	var buf bytes.Buffer
	c.SetOut(&buf)
	if err := c.Execute(); err != nil {
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
	cmdpkg.BuildVersion = "2.1.0"
	cmdpkg.BuildCommit = "deadbeef"

	c := newTestCommand()
	c.SetArgs([]string{"version", "--short"})
	var buf bytes.Buffer
	c.SetOut(&buf)
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := strings.TrimSpace(buf.String())
	if output != "2.1.0" {
		t.Errorf("expected '2.1.0', got %q", output)
	}
}
