// Package host provides platform-specific host setup and check operations.
package host

// SetupFlags holds configuration for host setup.
type SetupFlags struct {
	NoSleep      bool
	FirewallRule string
}

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
func Setup(flags SetupFlags) (SetupResult, error) {
	return setupImpl(flags)
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
func Check() (CheckResult, error) {
	return checkImpl()
}

// TailscaleIP returns the Tailscale IPv4 address, or empty string on failure.
func TailscaleIP() string {
	return tailscaleIPImpl()
}

// IsElevated returns true if the current process has admin/root privileges.
func IsElevated() bool {
	return isElevatedImpl()
}
