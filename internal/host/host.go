// Package host provides platform-specific host setup and check operations.
package host

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

// SetupFlags holds configuration for host setup (legacy alias).
// Deprecated: use Config or Flags instead.
type SetupFlags = Flags

// SetupResult holds the result of a host setup operation.
type SetupResult struct {
	RDPPort    int
	ControlURL string // the control plane for the descriptor: a URL, ControlPlaneSaaS, or "" when unknown
	Steps      []SetupStep
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
	result := setupOn(runtime.GOOS, cfg, execRunner{})
	logSetupSteps(logger, result.Steps)
	return result, nil
}

// CheckResult holds the result of a host check operation. RDPPort, RDPEnabled
// and FirewallOK are the pre-ADR-017 keys, derived from Services (§3).
type CheckResult struct {
	TailscaleIP string
	RDPPort     int
	RDPEnabled  bool
	FirewallOK  bool
	TailscaleUp bool
	ControlURL  string // as SetupResult.ControlURL
	// Elevated is whether the check ran as root or Administrator; without it
	// some service state cannot be read, so "off" and "unread" differ.
	Elevated bool
	// Published maps "kind:port" to whether host serve publishes that service;
	// nil when no host.mesh.health_addr was probed.
	Published map[string]bool
	Services  []Service
	Steps     []SetupStep
}

// ServiceRDP is the remote-desktop service kind (xrdp on Linux, RDP on Windows).
const ServiceRDP = "rdp"

// Service is the read state of one remote-access service the host declares.
// Active is nil when the service state cannot be read: on Linux when systemctl
// does not answer, on Windows until #455.
type Service struct {
	Kind       string
	Port       int
	Enabled    bool
	Active     *bool
	FirewallOK bool
	// Unread is true when a step could not read the service and called the
	// outcome unverified. Elevation is one possible cause; a missing systemctl or
	// a failed port read are others, and the step's fix names the actual one.
	Unread bool
}

// Check performs a read-only verification of host readiness. The resolved
// config supplies the firewall rule name and RDP port so the check matches the
// host's actual setup instead of assuming defaults.
// The logger may be nil — callers that don't need structured output can pass nil.
func Check(cfg Config, logger *slog.Logger) (CheckResult, error) {
	result := checkOn(runtime.GOOS, cfg, execRunner{})
	logCheck(logger, result)
	return result, nil
}

// logCheck logs the check's result. rdp_port is the legacy key, set only by an
// enabled service, so services names what the host declares.
// The logger may be nil.
func logCheck(logger *slog.Logger, result CheckResult) {
	if logger == nil {
		return
	}
	declared := make([]string, 0, len(result.Services))
	for _, svc := range result.Services {
		declared = append(declared, fmt.Sprintf("%s:%d", svc.Kind, svc.Port))
	}
	logger.Info("host check",
		"tailscale_up", result.TailscaleUp,
		"rdp_port", result.RDPPort,
		"rdp_enabled", result.RDPEnabled,
		"firewall_ok", result.FirewallOK,
		"services", strings.Join(declared, ","),
		"elevated", result.Elevated,
	)
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
	return sanitizeRDPRule(name)
}
