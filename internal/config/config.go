package config

import (
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"ts-bridge/internal/controlurl"
)

// logger is the package-level logger set by LoggerInit in main.go.
// When nil, warnings fall back to fmt.Fprintf(os.Stderr, ...).
var logger *slog.Logger

// SetLogger configures the package-level logger for structured warnings.
func SetLogger(l *slog.Logger) {
	logger = l
}

// warnUnknownField logs an unknown YAML field warning via the package logger,
// falling back to stderr if the logger is not yet initialized.
func warnUnknownField(field string) {
	if logger != nil {
		logger.Warn("unknown YAML field", "field", field)
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: unknown YAML field %q — ignoring\n", field)
	}
}

// warnPermission logs a permission warning via the package logger,
// falling back to stderr if the logger is not yet initialized.
func warnPermission(path string, perm uint32) {
	if logger != nil {
		logger.Warn("config file has loose permissions",
			"path", path, "perms", fmt.Sprintf("%o", perm))
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: %s has loose permissions (%04o); consider chmod 600 %s\n",
			path, perm, path)
	}
}

// warnRelativeStateDir warns when the resolved state directory is CWD-relative
// — an explicit relative override that risks leaking the tsnet node identity
// into the working directory (and any git tree that auto-commits it) (#207).
func warnRelativeStateDir(dir string) {
	if !stateDirIsCWDRelative(dir) {
		return
	}
	const msg = "state directory is relative to the current working directory; " +
		"the tsnet node identity (tailscaled.state) may leak into it and any git tree " +
		"that auto-commits it — prefer an absolute --state-dir"
	if logger != nil {
		logger.Warn(msg, "path", dir)
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: %s (path %q)\n", msg, dir)
	}
}

// warnEnvVar logs an environment variable parsing/validation warning via the
// package logger, falling back to stderr if the logger is not yet initialized.
func warnEnvVar(key, value, reason string) {
	if logger != nil {
		logger.Warn("env var invalid",
			"key", key, "value", value, "reason", reason)
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: %s=%q is not valid: %s\n", key, value, reason)
	}
}

const (
	// Default runtime values.
	defaultLocalAddr       = "127.0.0.1:33389"
	defaultHostname        = "ts-bridge"
	defaultAutoPortRange   = "33389-34388"
	defaultTimeout         = 30 * time.Second
	defaultDrainTimeout    = 15 * time.Second
	defaultMaxConnections  = 1000
	defaultDialRetries     = 3
	defaultDialBackoffBase = 1 * time.Second
	defaultDialBackoffMax  = 30 * time.Second
	defaultDialTimeout     = 5 * time.Second
	defaultBootstrapSOCKS  = "127.0.0.1:1055"
)

// Config holds the bridge configuration.
type Config struct {
	LocalAddr          string
	SOCKS5Addr         string
	SOCKS5Routes       map[string]string
	Target             string
	AuthKey            string // #nosec G117 -- internal struct, never serialized
	Hostname           string
	StateDir           string
	ControlURL         string
	BootstrapSSH       string
	BootstrapSOCKSAddr string
	ConnectTimeout     time.Duration
	DialTimeout        time.Duration
	DrainTimeout       time.Duration
	IdleTimeout        time.Duration
	DialRetries        int
	DialBackoffBase    time.Duration
	DialBackoffMax     time.Duration
	MaxConnections     int64
	HealthAddr         string
	Verbose            bool
	Quiet              bool
	LogFormat          string
	AutoInstance       bool
	EphemeralState     bool
}

// ValidateControlPlane routes the auth key to the right control plane by
// prefix (#209):
//
//   - A Headscale key (hskey-) with no control URL is rejected: it would
//     otherwise be sent to the default Tailscale SaaS control plane, which
//     rejects it with a misleading "API key does not exist" and a remediation
//     pointing at the wrong fix.
//   - A Tailscale key (tskey-) needs no control URL — it routes to SaaS by
//     default — and a tskey- with a custom control URL is left alone, since it
//     may be a legitimate self-hosted Tailscale-compatible control plane.
//   - Any non-empty control URL must pass controlurl.Validate (an http(s) URL
//     with a host and no user info), so a typo (e.g. a host with no scheme) is
//     caught here rather than failing deeper in tsnet with a less obvious
//     error, and user info (user:password@) never reaches the banner or the
//     logs. A query, path or fragment is not refused: a token placed there
//     would reach both, an accepted risk since no documented control plane
//     takes one there.
//
// Exported so every writer of a configuration file can enforce the pairing the
// runtime will enforce when it reads that file back: a producer that accepts
// what its own consumer rejects is the defect UX-355's review found in init.
func ValidateControlPlane(authKey, controlURL string) error {
	trimmed := strings.TrimSpace(controlURL)
	if strings.HasPrefix(authKey, "hskey-") && trimmed == "" {
		return errors.New("Headscale auth key (hskey-) requires a control URL; " +
			"set TS_CONTROL_URL (or --control-url) to your Headscale server, e.g. https://headscale.example.com")
	}
	if trimmed != "" {
		if err := controlurl.Validate(trimmed); err != nil {
			// The URL is not echoed: it may carry credentials (#618).
			return fmt.Errorf("control URL: %w", err)
		}
	}
	return nil
}

// parseBoolEnv parses a boolean from an environment variable.
// Accepted truthy values: "1", "true", "yes", "on" (case-insensitive).
// While Go's stdlib strconv.ParseBool only accepts "1", "t", "T", "true", "TRUE",
// "True", "0", "f", "F", "false", "FALSE", "False", we also accept "yes" and "on"
// because they match common shell/env-variable conventions and user expectations.
func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func deriveAutoLocalAddr(target, instanceName, portRange string) (string, error) {
	start, end, err := parsePortRange(portRange)
	if err != nil {
		return "", err
	}

	hostName, err := os.Hostname()
	if err != nil || hostName == "" {
		hostName = "unknown-host"
	}

	seed := fmt.Sprintf("%s|%s|%s", hostName, target, instanceName)
	port, err := selectAvailablePort(seed, start, end)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("127.0.0.1:%d", port), nil
}

func deriveAutoHostname(target, instanceName string) string {
	hostName, err := os.Hostname()
	if err != nil || hostName == "" {
		hostName = "unknown-host"
	}

	machine := SanitizeHostnameLabel(hostName)
	instance := SanitizeHostnameLabel(instanceName)
	if instance == "" {
		instance = machine
	}
	if instance == "" {
		instance = "bridge"
	}

	base := "tsb-" + instance
	if len(base) > 30 {
		base = strings.Trim(base[:30], "-")
	}
	if base == "" {
		base = "tsb-bridge"
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(machine + "|" + target + "|" + instanceName))
	hash := fmt.Sprintf("%06x", hasher.Sum32()&0xffffff)

	hostname := fmt.Sprintf("%s-%s-%d", base, hash, os.Getpid())
	if len(hostname) > 63 {
		hostname = strings.Trim(hostname[:63], "-")
	}
	if hostname == "" {
		return defaultHostname
	}
	return hostname
}

// SanitizeHostnameLabel returns a valid Tailscale hostname label:
// lowercase alphanumeric and hyphens only, no leading/trailing dashes.
func SanitizeHostnameLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	previousDash := false

	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			previousDash = false
			continue
		}
		if !previousDash {
			b.WriteByte('-')
			previousDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}

func parsePortRange(value string) (int, int, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("TS_PORT_RANGE invalid format %q (expected START-END)", value)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("TS_PORT_RANGE invalid start port: %w", err)
	}

	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("TS_PORT_RANGE invalid end port: %w", err)
	}

	if start < 1 || end > 65535 || start > end {
		return 0, 0, fmt.Errorf("TS_PORT_RANGE out of bounds: %d-%d", start, end)
	}

	return start, end, nil
}

var selectAvailablePort = selectAvailablePortImpl

func selectAvailablePortImpl(seed string, start, end int) (int, error) {
	span := end - start + 1
	if span <= 0 {
		return 0, fmt.Errorf("TS_PORT_RANGE has invalid span: %d", span)
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(seed))
	offset := int(int64(hasher.Sum32()) % int64(span))

	for i := 0; i < span; i++ {
		port := start + ((offset + i) % span)
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		if err := listener.Close(); err != nil {
			continue
		}
		return port, nil
	}

	return 0, fmt.Errorf("TS_PORT_RANGE has no free ports in %d-%d", start, end)
}
