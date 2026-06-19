// Package host provides platform-specific host setup and check operations.
package host

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Default values for host configuration.
const (
	defaultRDPPort          = 3389
	defaultFirewallRuleName = "Tailscale-RDP-Ingress"
	defaultNoSleep          = false
	defaultVerbose          = false
	defaultLogFormat        = "text"
	logFormatJSON           = "json"
)

// DefaultRDPPort returns the default RDP port.
func DefaultRDPPort() int { return defaultRDPPort }

// DefaultFirewallRule returns the default firewall rule name.
func DefaultFirewallRule() string { return defaultFirewallRuleName }

// DefaultNoSleep returns the default for the no-sleep setting.
func DefaultNoSleep() bool { return defaultNoSleep }

// DefaultVerbose returns the default for verbose logging.
func DefaultVerbose() bool { return defaultVerbose }

// DefaultLogFormat returns the default log format.
func DefaultLogFormat() string { return defaultLogFormat }

// Flags holds values provided via CLI flags for host commands.
//
// NoSleepSet / VerboseSet record whether the corresponding bool flag was
// explicitly passed on the command line. Cobra defaults a bool flag to false,
// so the value alone can't distinguish "not set" from "explicitly false";
// without the *Set companion an unset flag would clobber the env layer and
// break the flags > env > defaults precedence chain. Non-bool flags use a
// natural sentinel instead (Port == 0, FirewallRule/LogFormat == "").
type Flags struct {
	FirewallRule string
	NoSleep      bool
	NoSleepSet   bool
	Port         int
	Verbose      bool
	VerboseSet   bool
	LogFormat    string
}

// Config holds the resolved host configuration after merge.
type Config struct {
	FirewallRule string
	NoSleep      bool
	Port         int
	Verbose      bool
	LogFormat    string
}

// Merge applies the precedence chain: flags > env > defaults.
// Returns a fully-populated Config.
func Merge(flags Flags) Config {
	cfg := defaults()
	applyEnv(&cfg)
	applyFlags(&cfg, flags)
	return cfg
}

// defaults returns a Config with built-in defaults.
func defaults() Config {
	return Config{
		FirewallRule: defaultFirewallRuleName,
		NoSleep:      defaultNoSleep,
		Port:         defaultRDPPort,
		Verbose:      defaultVerbose,
		LogFormat:    defaultLogFormat,
	}
}

// LoadConfig reads configuration from environment variables only (no flags).
// Used by the init wizard which needs to read existing env values before
// writing new ones. Invalid values produce warnings and fall back to defaults.
func LoadConfig() (Config, error) {
	cfg := defaults()
	applyEnv(&cfg)

	// Validate firewall rule name (hard — invalid names can't be sanitized).
	if _, err := sanitizeFirewallRule(cfg.FirewallRule); err != nil {
		return Config{}, fmt.Errorf("TS_HOST_FIREWALL_RULE invalid: %w", err)
	}

	return cfg, nil
}

func applyEnv(cfg *Config) {
	// Firewall rule.
	if v := os.Getenv("TS_HOST_FIREWALL_RULE"); v != "" {
		cfg.FirewallRule = v
	}

	// NoSleep.
	if v := os.Getenv("TS_HOST_NO_SLEEP"); v != "" {
		cfg.NoSleep = parseBoolEnv(v)
	}

	// RDP Port.
	if v := os.Getenv("TS_HOST_RDP_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			warnHostEnvVar("TS_HOST_RDP_PORT", v, "invalid integer")
		} else if port < 1 || port > 65535 {
			warnHostEnvVar("TS_HOST_RDP_PORT", v, "must be between 1 and 65535")
		} else {
			cfg.Port = port
		}
	}

	// Verbose.
	if v := os.Getenv("TS_HOST_VERBOSE"); v != "" {
		cfg.Verbose = parseBoolEnv(v)
	}

	// Log format. Invalid values warn and fall back to the existing default.
	if v := os.Getenv("TS_HOST_LOG_FORMAT"); v != "" {
		format := strings.ToLower(v)
		if format != defaultLogFormat && format != logFormatJSON {
			warnHostEnvVar("TS_HOST_LOG_FORMAT", v, "must be 'text' or 'json'")
		} else {
			cfg.LogFormat = format
		}
	}
}

func applyFlags(cfg *Config, flags Flags) {
	// Firewall rule — apply if non-empty.
	if flags.FirewallRule != "" {
		cfg.FirewallRule = flags.FirewallRule
	}

	// NoSleep — apply only if the flag was explicitly passed, so an unset
	// --no-sleep does not overwrite TS_HOST_NO_SLEEP from the environment.
	if flags.NoSleepSet {
		cfg.NoSleep = flags.NoSleep
	}

	// RDP Port — apply only if positive (zero means "not set").
	if flags.Port > 0 {
		cfg.Port = flags.Port
	}

	// Verbose — apply only if explicitly passed (same reasoning as NoSleep).
	if flags.VerboseSet {
		cfg.Verbose = flags.Verbose
	}

	// Log format — apply if non-empty.
	if flags.LogFormat != "" {
		cfg.LogFormat = flags.LogFormat
	}
}

// warnHostEnvVar logs a host configuration env var warning to stderr.
func warnHostEnvVar(key, value, reason string) {
	fmt.Fprintf(os.Stderr, "WARNING: %s=%q is not valid: %s\n", key, value, reason)
}

// parseBoolEnv parses a boolean from a string value.
// Accepted truthy values: "1", "true", "yes", "on" (case-insensitive).
func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
