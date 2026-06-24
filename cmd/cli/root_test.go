package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newErrorDemoCmd builds a throwaway command configured the same way the real
// root is, so the error-output behavior can be exercised in isolation.
func newErrorDemoCmd(runErr error) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "demo",
		RunE: func(*cobra.Command, []string) error { return runErr },
	}
	cmd.Flags().String("flag", "", "a flag")
	configureUsageOnErrors(cmd)
	return cmd
}

// A runtime (RunE) failure must print only the error — never the usage block (#210).
func TestConfigureUsageOnErrors_RuntimeErrorHidesUsage(t *testing.T) {
	cmd := newErrorDemoCmd(errors.New("backend: invalid key"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected the RunE error to propagate")
	}
	if strings.Contains(out.String(), "Usage:") {
		t.Errorf("runtime error must not dump the usage block, got:\n%s", out.String())
	}
	// cobra must stay silent about the error (main prints it) — no double print.
	if strings.Contains(out.String(), "Error:") {
		t.Errorf("cobra should not print the error itself (SilenceErrors), got:\n%s", out.String())
	}
}

// An argument/flag-parsing failure should still show usage — it's genuinely helpful there.
func TestConfigureUsageOnErrors_FlagErrorShowsUsage(t *testing.T) {
	cmd := newErrorDemoCmd(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--nonexistent-flag"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a flag-parse error")
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("flag-parse error should show usage, got:\n%s", out.String())
	}
}
