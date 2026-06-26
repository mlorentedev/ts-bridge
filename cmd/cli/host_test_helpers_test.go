// Package cmd provides the Cobra CLI for ts-bridge.
//
// This file contains test-only exports that allow cmd/cli/host_test.go
// (an external test package, cmd_test) to access internal functions.
// Only compiled during `go test` due to the _test.go suffix.
package cmd

import (
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
func PrintSetupJSONForTest(result host.SetupResult) error {
	return printSetupJSON(result)
}

// PrintCheckJSONForTest is a test-only wrapper for printCheckJSON.
func PrintCheckJSONForTest(r host.CheckResult) error {
	return printCheckJSON(r)
}
