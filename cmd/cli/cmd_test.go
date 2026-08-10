package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	cmdpkg "ts-bridge/cmd/cli"
)

const (
	defaultBuildVersion = "dev"
	defaultBuildCommit  = "unknown"
)

func TestRootHelp(t *testing.T) {
	cmdpkg.BuildVersion = defaultBuildVersion
	cmdpkg.BuildCommit = defaultBuildCommit

	c := cmdpkg.NewRootCmd()
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

func TestNewRootCmdCreatesIndependentTrees(t *testing.T) {
	first := cmdpkg.NewRootCmd()
	second := cmdpkg.NewRootCmd()

	if first == second {
		t.Fatal("NewRootCmd returned the same command instance")
	}
	if err := first.PersistentFlags().Set("verbose", "true"); err != nil {
		t.Fatalf("set first --verbose: %v", err)
	}

	secondVerbose := second.PersistentFlags().Lookup("verbose")
	if secondVerbose == nil {
		t.Fatal("second tree does not define --verbose")
	}
	if secondVerbose.Changed {
		t.Error("second tree inherited the first tree's --verbose Changed state")
	}
	if got := secondVerbose.Value.String(); got != "false" {
		t.Errorf("second tree inherited --verbose=%s, want false", got)
	}
}

func TestNewRootCmdContainsProductionCommands(t *testing.T) {
	root := cmdpkg.NewRootCmd()
	got := make(map[string]bool, len(root.Commands()))
	for _, command := range root.Commands() {
		got[command.Name()] = true
	}

	want := []string{"connect", "discover", "host", "import", "init", "status", "version"}
	for _, name := range want {
		if !got[name] {
			t.Errorf("production root is missing %q", name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("production root has %d commands, want %d", len(got), len(want))
	}
}

func TestVersionCommandDefault(t *testing.T) {
	cmdpkg.BuildVersion = defaultBuildVersion
	cmdpkg.BuildCommit = defaultBuildCommit

	c := cmdpkg.NewRootCmd()
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

	c := cmdpkg.NewRootCmd()
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

	c := cmdpkg.NewRootCmd()
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

	c := cmdpkg.NewRootCmd()
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
