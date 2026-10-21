// Package host provides platform-specific host setup and check operations.
package host

import (
	"log/slog"
	"runtime"
)

// SetupFlags holds configuration for host setup (legacy alias).
// Deprecated: use Config or Flags instead.
type SetupFlags = Flags

// SetupResult holds the result of a host setup operation.
type SetupResult struct {
	RDPPort int
	Steps   []SetupStep
}

// SetupStep describes a single setup step and its outcome. Success is
// Outcome.Succeeded(), kept for the existing JSON contract.
type SetupStep struct {
	Name    string
	Outcome Outcome
	Success bool
	Message string
	Fix     string // the action that would resolve a non-success outcome
}

// Setup performs platform-specific host setup.
// The logger may be nil — callers that don't need structured output can pass nil.
func Setup(cfg Config, logger *slog.Logger) (SetupResult, error) {
	result := setupFor(runtime.GOOS, cfg, execRunner{})
	logSetupSteps(logger, result.Steps)
	return result, nil
}

// CheckResult holds the result of a host check operation.
type CheckResult struct {
	TailscaleIP string
	RDPPort     int
	RDPEnabled  bool
	FirewallOK  bool
	TailscaleUp bool
}

// Check performs a read-only verification of host readiness. The resolved
// config supplies the firewall rule name and RDP port so the check matches the
// host's actual setup instead of assuming defaults.
// The logger may be nil — callers that don't need structured output can pass nil.
func Check(cfg Config, logger *slog.Logger) (CheckResult, error) {
	result := checkFor(runtime.GOOS, cfg, execRunner{})
	if logger != nil {
		logger.Info("host check",
			"tailscale_up", result.TailscaleUp,
			"rdp_port", result.RDPPort,
			"rdp_enabled", result.RDPEnabled,
			"firewall_ok", result.FirewallOK,
		)
	}
	return result, nil
}

// TailscaleIP returns the Tailscale IPv4 address, or empty string on failure.
func TailscaleIP() string {
	return tailscaleIP(execRunner{})
}

// IsElevated returns true if the current process has admin/root privileges.
func IsElevated() bool {
	return isElevatedImpl()
}

// SanitizeFirewallRule validates a firewall rule name.
// Exported for use by cmd/cli/host.go for early validation.
func SanitizeFirewallRule(name string) (string, error) {
	return sanitizeFirewallRule(name)
}
