package config

import (
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// Default runtime values.
	defaultLocalAddr     = "127.0.0.1:33389"
	defaultHostname      = "ts-bridge"
	defaultStateDir      = "./ts-state"
	defaultAutoPortRange = "33389-34388"
	defaultTimeout       = 30 * time.Second
	defaultDrainTimeout  = 15 * time.Second
	defaultMaxConnections = 1000
)

// Config holds the bridge configuration.
type Config struct {
	LocalAddr      string
	Target         string
	AuthKey        string // #nosec G117 -- internal struct, never serialized
	Hostname       string
	StateDir       string
	ControlURL     string
	ConnectTimeout time.Duration
	DrainTimeout   time.Duration
	MaxConnections int64
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

	timeout := defaultTimeout
	if t := os.Getenv("TS_TIMEOUT"); t != "" {
		d, err := time.ParseDuration(t)
		if err != nil {
			return Config{}, fmt.Errorf("TS_TIMEOUT invalid: %w", err)
		}
		timeout = d
	}

	drainTimeout := defaultDrainTimeout
	if t := os.Getenv("TS_DRAIN_TIMEOUT"); t != "" {
		d, err := time.ParseDuration(t)
		if err != nil {
			return Config{}, fmt.Errorf("TS_DRAIN_TIMEOUT invalid: %w", err)
		}
		drainTimeout = d
	}

	maxConns := int64(defaultMaxConnections)
	if m := os.Getenv("TS_MAX_CONNECTIONS"); m != "" {
		n, err := strconv.ParseInt(m, 10, 64)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("TS_MAX_CONNECTIONS invalid: %w", err)
		}
		maxConns = n
	}

	verbose := verboseFlag || os.Getenv("TS_VERBOSE") == "true" || os.Getenv("TS_VERBOSE") == "1"

	cfg := Config{
		LocalAddr:      os.Getenv("TS_LOCAL_ADDR"),
		Target:         target,
		AuthKey:        authKey,
		Hostname:       os.Getenv("TS_HOSTNAME"),
		StateDir:       os.Getenv("TS_STATE_DIR"),
		ControlURL:     os.Getenv("TS_CONTROL_URL"),
		ConnectTimeout: timeout,
		DrainTimeout:   drainTimeout,
		MaxConnections: maxConns,
		HealthAddr:     os.Getenv("TS_HEALTH_ADDR"),
		Verbose:        verbose,
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
		return "", errors.New("TS_AUTHKEY: invalid format (must start with tskey- or hskey-)")
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

	machine := sanitizeHostnameLabel(hostName)
	instance := sanitizeHostnameLabel(instanceName)
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

func sanitizeHostnameLabel(value string) string {
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
