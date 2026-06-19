// Package host provides platform-specific host setup and check operations.
package host

import "log/slog"

// SetupFlags holds configuration for host setup (legacy alias).
// Deprecated: use Config or Flags instead.
type SetupFlags = Flags

// SetupResult holds the result of a host setup operation.
type SetupResult struct {
	RDPPort      int
	Steps        []SetupStep
}

// SetupStep describes a single setup step and its outcome.
type SetupStep struct {
	Name    string
	Success bool
	Message string
}

// Setup performs platform-specific host setup.
// The logger may be nil — callers that don't need structured output can pass nil.
func Setup(cfg Config, logger *slog.Logger) (SetupResult, error) {
	return setupImpl(cfg, logger)
}

// CheckResult holds the result of a host check operation.
type CheckResult struct {
	TailscaleIP  string
	RDPPort      int
	RDPEnabled   bool
	FirewallOK   bool
	TailscaleUp  bool
}

// Check performs a read-only verification of host readiness.
// The logger may be nil — callers that don't need structured output can pass nil.
func Check(logger *slog.Logger) (CheckResult, error) {
	return checkImpl(logger)
}

// TailscaleIP returns the Tailscale IPv4 address, or empty string on failure.
func TailscaleIP() string {
	return tailscaleIPImpl()
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
