// Package cmd provides the Cobra CLI for ts-bridge.
//
// This file contains test-only exports that allow cmd/cli/host_test.go
// (an external test package, cmd_test) to access internal functions.
// Only compiled during `go test` due to the _test.go suffix.
package cmd

import (
	"github.com/spf13/cobra"
	"io"
	"log/slog"

	"ts-bridge/internal/host"
)

// BuildDescriptorLineForTest is a test-only wrapper for buildDescriptorLine.
func BuildDescriptorLineForTest(ip string, port int, controlURL string) string {
	return buildDescriptorLine(ip, port, controlURL)
}

// ─── Test-only exports ───────────────────────────────────────────

// HostInitFlagsForTest is a test-only alias for hostInitFlags.
type HostInitFlagsForTest = hostInitFlags

// WriteHostEnvForTest is a test-only wrapper for writeHostEnv.
func WriteHostEnvForTest(f hostInitFlags) error {
	return writeHostEnv(f)
}

// SetupJSONOutputForTest is a test-only alias for setupJSONOutput.
type SetupJSONOutputForTest = setupJSONOutput

// SetupStepJSONForTest is a test-only alias for setupStepJSON.
type SetupStepJSONForTest = setupStepJSON

// PrintSetupJSONForTest is a test-only wrapper for printSetupJSON.
func PrintSetupJSONForTest(w io.Writer, result host.SetupResult) error {
	return printSetupJSON(w, result)
}

// PrintCheckJSONForTest is a test-only wrapper for printCheckJSON.
func PrintCheckJSONForTest(w io.Writer, r host.CheckResult) error {
	return printCheckJSON(w, r)
}

// HostInitLoggerForTest is a test-only wrapper for hostInitLogger. It exercises
// the production wiring, including which stream the logger targets.
func HostInitLoggerForTest(cfg host.Config) *slog.Logger {
	return hostInitLogger(cfg)
}

// NewHostLoggerForTest is a test-only wrapper for newHostLogger.
func NewHostLoggerForTest(cfg host.Config, w io.Writer) *slog.Logger {
	return newHostLogger(cfg, w)
}

// CheckJSONForTest is a test-only wrapper for checkJSON.
func CheckJSONForTest(r host.CheckResult) ([]byte, error) {
	return checkJSON(r)
}

// SetupJSONForTest is a test-only wrapper for setupJSON.
func SetupJSONForTest(result host.SetupResult) ([]byte, error) {
	return setupJSON(result)
}

// CheckConvergenceForTest is a test-only wrapper for checkConvergence.
func CheckConvergenceForTest(steps []host.SetupStep) error {
	return checkConvergence(steps)
}

// HostCheckCmdForTest returns the host check command running check instead of
// host.Check, and restores host.Check when the test ends.
func HostCheckCmdForTest(t interface{ Cleanup(func()) }, check func(host.Config, *slog.Logger) (host.CheckResult, error)) *cobra.Command {
	prev := hostCheck
	hostCheck = check
	t.Cleanup(func() { hostCheck = prev })
	return newHostCheckCmd()
}

// HostSetupCmdForTest returns a host setup command that runs setup instead of
// host.Setup, as on Linux, and treats the process as elevated or not.
func HostSetupCmdForTest(t interface{ Cleanup(func()) }, elevated bool, setup func(host.Config, *slog.Logger) (host.SetupResult, error)) *cobra.Command {
	prevSetup, prevElevated, prevGOOS := hostSetup, hostIsElevated, hostGOOS
	hostSetup = setup
	hostIsElevated = func() bool { return elevated }
	hostGOOS = "linux"
	t.Cleanup(func() { hostSetup, hostIsElevated, hostGOOS = prevSetup, prevElevated, prevGOOS })
	c := newHostSetupCmd()
	c.SilenceUsage = true // as the root command sets it (#210)
	return c
}
