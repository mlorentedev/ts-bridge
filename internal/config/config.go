package config

import (
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	defaultLocalAddr     = "127.0.0.1:33389"
	defaultHostname      = "ts-bridge"
	defaultStateDir      = "./ts-state"
	defaultAutoPortRange = "33389-34388"
	defaultTimeout       = 30 * time.Second
	defaultDrainTimeout  = 15 * time.Second
	defaultMaxConnections = 1000
	defaultDialRetries     = 3
	defaultDialBackoffBase = 1 * time.Second
	defaultDialBackoffMax  = 30 * time.Second
	defaultDialTimeout     = 5 * time.Second
)

// Config holds the bridge configuration.
type Config struct {
	LocalAddr      string
	Target         string
	AuthKey        string // #nosec G117 -- internal struct, never serialized
	Hostname       string
	StateDir       string
	ControlURL     string
	ConnectTimeout  time.Duration
	DialTimeout     time.Duration
	DrainTimeout    time.Duration
	IdleTimeout     time.Duration
	DialRetries     int
	DialBackoffBase time.Duration
	DialBackoffMax  time.Duration
	MaxConnections  int64
	HealthAddr     string
	Verbose        bool
	LogFormat      string
	AutoInstance   bool
	EphemeralState bool
}

// LoadConfig parses environment variables into a Config struct.
func LoadConfig(verboseFlag bool) (Config, error) {
	target, err := parseTarget()
	if err != nil {
		return Config{}, err
	}

	authKey, err := parseAuthKey()
	if err != nil {
		return Config{}, err
	}

	timeout, err := parseDurationEnv("TS_TIMEOUT", defaultTimeout)
	if err != nil {
		return Config{}, err
	}

	drainTimeout, err := parseDurationEnv("TS_DRAIN_TIMEOUT", defaultDrainTimeout)
	if err != nil {
		return Config{}, err
	}

	idleTimeout, dialTimeout, err := parseTimeoutEnvs()
	if err != nil {
		return Config{}, err
	}

	dialRetries, dialBackoffBase, dialBackoffMax, err := parseDialConfig()
	if err != nil {
		return Config{}, err
	}

	maxConns, err := parseInt64Env("TS_MAX_CONNECTIONS", defaultMaxConnections)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		LocalAddr:      os.Getenv("TS_LOCAL_ADDR"),
		Target:         target,
		AuthKey:        authKey,
		Hostname:       os.Getenv("TS_HOSTNAME"),
		StateDir:       os.Getenv("TS_STATE_DIR"),
		ControlURL:     os.Getenv("TS_CONTROL_URL"),
		ConnectTimeout:  timeout,
		DialTimeout:     dialTimeout,
		DrainTimeout:    drainTimeout,
		IdleTimeout:     idleTimeout,
		DialRetries:     dialRetries,
		DialBackoffBase: dialBackoffBase,
		DialBackoffMax:  dialBackoffMax,
		MaxConnections:  maxConns,
		HealthAddr:     os.Getenv("TS_HEALTH_ADDR"),
		Verbose:        verboseFlag || parseBoolEnv(os.Getenv("TS_VERBOSE")),
		LogFormat:      EnvOr("TS_LOG_FORMAT", "text"),
	}

	if err := applyAutoInstanceConfig(&cfg); err != nil {
		return Config{}, err
	}

	if cfg.LocalAddr == "" {
		cfg.LocalAddr = defaultLocalAddr
	}
	if cfg.Hostname == "" {
		cfg.Hostname = defaultHostname
	}
	if cfg.StateDir == "" {
		cfg.StateDir = defaultStateDir
	}

	return cfg, nil
}

func parseDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s invalid: %w", key, err)
	}
	return d, nil
}

func parseInt64Env(key string, fallback int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s invalid: %w", key, err)
	}
	return n, nil
}

// parseDialRetries is separate from parseInt64Env because retries may legitimately
// be zero (disables retry) while connection limits etc. require >= 1.
func parseDialRetries() (int, error) {
	v := os.Getenv("TS_DIAL_RETRIES")
	if v == "" {
		return defaultDialRetries, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("TS_DIAL_RETRIES invalid: %w", err)
	}
	if n < 0 {
		return 0, fmt.Errorf("TS_DIAL_RETRIES must be >= 0, got %d", n)
	}
	return n, nil
}

// parseTimeoutEnvs collects the two per-connection timeouts (idle + dial) so
// LoadConfig stays under the cyclomatic-complexity threshold. Idle accepts
// 0 (disabled); dial must be strictly positive.
func parseTimeoutEnvs() (idle, dial time.Duration, err error) {
	idle, err = parseDurationEnv("TS_IDLE_TIMEOUT", 0)
	if err != nil {
		return 0, 0, err
	}
	if idle < 0 {
		return 0, 0, fmt.Errorf("TS_IDLE_TIMEOUT must be >= 0, got %v", idle)
	}

	dial, err = parseDurationEnv("TS_DIAL_TIMEOUT", defaultDialTimeout)
	if err != nil {
		return 0, 0, err
	}
	if dial <= 0 {
		return 0, 0, fmt.Errorf("TS_DIAL_TIMEOUT must be > 0, got %v", dial)
	}

	return idle, dial, nil
}

// parseDialConfig collects the three ReconnectDialer parameters together so
// LoadConfig stays under the cyclomatic-complexity threshold.
func parseDialConfig() (retries int, base, maxBackoff time.Duration, err error) {
	retries, err = parseDialRetries()
	if err != nil {
		return 0, 0, 0, err
	}

	base, err = parseDurationEnv("TS_DIAL_BACKOFF_BASE", defaultDialBackoffBase)
	if err != nil {
		return 0, 0, 0, err
	}
	if base < 0 {
		return 0, 0, 0, fmt.Errorf("TS_DIAL_BACKOFF_BASE must be >= 0, got %v", base)
	}

	maxBackoff, err = parseDurationEnv("TS_DIAL_BACKOFF_MAX", defaultDialBackoffMax)
	if err != nil {
		return 0, 0, 0, err
	}
	if maxBackoff < 0 {
		return 0, 0, 0, fmt.Errorf("TS_DIAL_BACKOFF_MAX must be >= 0, got %v", maxBackoff)
	}
	if maxBackoff < base {
		return 0, 0, 0, fmt.Errorf("TS_DIAL_BACKOFF_MAX (%v) must be >= TS_DIAL_BACKOFF_BASE (%v)", maxBackoff, base)
	}

	return retries, base, maxBackoff, nil
}



func parseTarget() (string, error) {
	target := os.Getenv("TS_TARGET")
	if target == "" {
		return "", errors.New("TS_TARGET is required (format: HOST:PORT)")
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return "", fmt.Errorf("TS_TARGET invalid format: %w", err)
	}
	if host == "" {
		return "", errors.New("TS_TARGET: host cannot be empty")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("TS_TARGET: invalid port %q", portStr)
	}
	return target, nil
}

func parseAuthKey() (string, error) {
	authKey := os.Getenv("TS_AUTHKEY")
	if authKey == "" {
		return "", errors.New("TS_AUTHKEY is required")
	}
	if !strings.HasPrefix(authKey, "tskey-") && !strings.HasPrefix(authKey, "hskey-") {
		hint := ""
		if strings.HasPrefix(authKey, "http://") || strings.HasPrefix(authKey, "https://") {
			hint = " (did you paste a Tailscale login URL instead of the auth key? use the key value after /auth/singleusekey/ or /auth/key)"
		}
		return "", errors.New("TS_AUTHKEY: invalid format (must start with tskey- or hskey-)" + hint)
	}
	return authKey, nil
}

// EnvOr returns the environment variable or a fallback.
func EnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func applyAutoInstanceConfig(cfg *Config) error {
	cfg.AutoInstance = shouldEnableAutoInstance()
	if !cfg.AutoInstance {
		return nil
	}

	instanceName := os.Getenv("TS_INSTANCE_NAME")
	portRange := EnvOr("TS_PORT_RANGE", defaultAutoPortRange)

	if cfg.LocalAddr == "" {
		localAddr, err := deriveAutoLocalAddr(cfg.Target, instanceName, portRange)
		if err != nil {
			return err
		}
		cfg.LocalAddr = localAddr
	}

	if cfg.Hostname == "" {
		cfg.Hostname = deriveAutoHostname(cfg.Target, instanceName)
	}

	if cfg.StateDir == "" {
		cfg.StateDir = filepath.Join(os.TempDir(), "ts-bridge", cfg.Hostname)
		cfg.EphemeralState = true
	}

	return nil
}

func shouldEnableAutoInstance() bool {
	if parseBoolEnv(os.Getenv("TS_MANUAL_MODE")) {
		return false
	}

	rawAutoMode := strings.TrimSpace(os.Getenv("TS_AUTO_INSTANCE"))
	if rawAutoMode == "" {
		return true
	}

	return parseBoolEnv(rawAutoMode)
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

func selectAvailablePort(seed string, start, end int) (int, error) {
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
