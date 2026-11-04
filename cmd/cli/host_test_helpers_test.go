// Package cmd provides the Cobra CLI for ts-bridge.
//
// This file contains test-only exports that allow cmd/cli/host_test.go
// (an external test package, cmd_test) to access internal functions.
// Only compiled during `go test` due to the _test.go suffix.
package cmd

import (
	"context"
	"io"
	"log/slog"

	"github.com/spf13/cobra"

	"ts-bridge/internal/bootstrapssh"
	"ts-bridge/internal/host"
	"ts-bridge/internal/serve"
)

// BuildDescriptorLineForTest is a test-only wrapper for buildDescriptorLine.
func BuildDescriptorLineForTest(ip string, port int, controlURL string) string {
	return buildDescriptorLine(ip, port, controlURL)
}

// ─── Test-only exports ───────────────────────────────────────────

// HostInitCmdForTest returns a host init command as on goos.
func HostInitCmdForTest(t interface{ Cleanup(func()) }, goos string) *cobra.Command {
	prev := hostGOOS
	hostGOOS = goos
	t.Cleanup(func() { hostGOOS = prev })
	c := newHostInitCmd()
	c.SilenceUsage = true
	return c
}

// SetupJSONOutputForTest is a test-only alias for setupJSONOutput.
type SetupJSONOutputForTest = setupJSONOutput

// SetupStepJSONForTest is a test-only alias for setupStepJSON.
type SetupStepJSONForTest = setupStepJSON

// PrintSetupJSONForTest is a test-only wrapper for printSetupJSON.
func PrintSetupJSONForTest(w io.Writer, result host.SetupResult) error {
	return printSetupJSON(w, "", result)
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
	return setupJSON("", result)
}

// SetupJSONOnForTest is setupJSON for a host with the given Tailscale IP.
func SetupJSONOnForTest(tsIP string, result host.SetupResult) ([]byte, error) {
	return setupJSON(tsIP, result)
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

// HostCheckCmdOnForTest is HostCheckCmdForTest as goos.
func HostCheckCmdOnForTest(t interface{ Cleanup(func()) }, goos string, check func(host.Config, *slog.Logger) (host.CheckResult, error)) *cobra.Command {
	prevGOOS := hostGOOS
	hostGOOS = goos
	t.Cleanup(func() { hostGOOS = prevGOOS })
	c := HostCheckCmdForTest(t, check)
	c.SilenceUsage = true
	return c
}

// PrintSetupSummaryForTest is a test-only wrapper for printSetupSummary.
func PrintSetupSummaryForTest(w io.Writer, tsIP string, result host.SetupResult) {
	printSetupSummary(w, tsIP, result)
}

// HostSetupCmdForTest returns a host setup command that runs setup instead of
// host.Setup, as on goos, and treats the process as elevated or not.
func HostSetupCmdForTest(t interface{ Cleanup(func()) }, goos string, elevated bool, setup func(host.Config, *slog.Logger) (host.SetupResult, error)) *cobra.Command {
	prevSetup, prevElevated, prevGOOS := hostSetup, hostIsElevated, hostGOOS
	hostSetup = setup
	hostIsElevated = func() bool { return elevated }
	hostGOOS = goos
	t.Cleanup(func() { hostSetup, hostIsElevated, hostGOOS = prevSetup, prevElevated, prevGOOS })
	c := newHostSetupCmd()
	c.SilenceUsage = true // as the root command sets it (#210)
	return c
}

// SetHostTailscaleIPForTest makes setup's descriptor name ip for the test.
func SetHostTailscaleIPForTest(t interface{ Cleanup(func()) }, ip string) {
	prev := hostTailscaleIP
	hostTailscaleIP = func() string { return ip }
	t.Cleanup(func() { hostTailscaleIP = prev })
}

// PrintCheckTextForTest is a test-only wrapper for printCheckText.
func PrintCheckTextForTest(w io.Writer, r host.CheckResult) {
	printCheckText(w, r)
}

// ServeNodeForTest is the node a fake serve starter returns.
type ServeNodeForTest = serveNode

// ControlBootstrapForTest is the SSH bootstrap a fake tunnel starter returns.
type ControlBootstrapForTest = controlBootstrap

// SetServeBootstrapForTest replaces the SSH tunnel starter and the control
// proxy configurer for the test.
func SetServeBootstrapForTest(
	t interface{ Cleanup(func()) },
	start func(context.Context, bootstrapssh.Config) (ControlBootstrapForTest, error),
	configure func(context.Context, string, string) error,
) {
	prevStart, prevConfigure := sshTunnelStarter, controlProxyConfigurer
	sshTunnelStarter, controlProxyConfigurer = start, configure
	t.Cleanup(func() { sshTunnelStarter, controlProxyConfigurer = prevStart, prevConfigure })
}

// HostServeCmdForTest returns a host serve command, as on goos, that starts
// its node with start and checks the state directory with ensure.
func HostServeCmdForTest(
	t interface{ Cleanup(func()) },
	goos string,
	start func(context.Context, serve.NodeConfig) (ServeNodeForTest, []string, error),
	ensure func(string) error,
) *cobra.Command {
	prevStart, prevEnsure, prevGOOS, prevKey := serveNodeStarter, serveEnsureStateDir, hostGOOS, serveCheckKeyFile
	// Test key files live in the user's temp dir; the guard has its own tests.
	serveNodeStarter, serveEnsureStateDir, hostGOOS, serveCheckKeyFile = start, ensure, goos, func(string) error { return nil }
	t.Cleanup(func() {
		serveNodeStarter, serveEnsureStateDir, hostGOOS, serveCheckKeyFile = prevStart, prevEnsure, prevGOOS, prevKey
	})
	c := newHostServeCmd()
	c.SilenceUsage = true
	return c
}

// shippedServeKeyCheck is serveCheckKeyFile as the package initialized it,
// captured before any test stubs it.
var shippedServeKeyCheck = serveCheckKeyFile

// UseProductionServeKeyCheckForTest undoes HostServeCmdForTest's key-check
// stub, so the test runs the guard serve ships with.
func UseProductionServeKeyCheckForTest(t interface{ Cleanup(func()) }) {
	prev := serveCheckKeyFile
	serveCheckKeyFile = shippedServeKeyCheck
	t.Cleanup(func() { serveCheckKeyFile = prev })
}

// shippedServeEnsureStateDir is serveEnsureStateDir as the package initialized
// it, captured before any test stubs it.
var shippedServeEnsureStateDir = serveEnsureStateDir

// UseProductionServeStateDirCheckForTest undoes HostServeCmdForTest's
// state_dir stub, so the test runs the guard serve ships with.
func UseProductionServeStateDirCheckForTest(t interface{ Cleanup(func()) }) {
	prev := serveEnsureStateDir
	serveEnsureStateDir = shippedServeEnsureStateDir
	t.Cleanup(func() { serveEnsureStateDir = prev })
}
