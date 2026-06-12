package config

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// PartialConfig holds values loaded from a YAML config file.
type PartialConfig struct {
	Version         int           `yaml:"version"`
	Target          string        `yaml:"target"`
	Hostname        string        `yaml:"hostname"`
	ControlURL      string        `yaml:"control_url"`
	StateDir        string        `yaml:"state_dir"`
	HealthAddr      string        `yaml:"health_addr"`
	LogFormat       string        `yaml:"log_format"`
	AuthKey         string        `yaml:"auth_key"` // #nosec G117 -- internal struct, never serialized, explicitly rejected in YAML
	Timeout         time.Duration `yaml:"timeout"`
	DialTimeout     time.Duration `yaml:"dial_timeout"`
	DrainTimeout    time.Duration `yaml:"drain_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	DialRetries     int           `yaml:"dial_retries"`
	DialBackoffBase time.Duration `yaml:"dial_backoff_base"`
	DialBackoffMax  time.Duration `yaml:"dial_backoff_max"`
	MaxConnections  int64         `yaml:"max_connections"`
	AutoInstance    *bool         `yaml:"auto_instance"`
}

// FlagSet holds values provided via CLI flags.
type FlagSet struct {
	Target          string
	AuthKey         string // #nosec G117 -- internal struct, never serialized
	AuthKeyFile     string
	Instance        string
	LocalAddr       string
	Hostname        string
	StateDir        string
	ControlURL      string
	Timeout         time.Duration
	DialTimeout     time.Duration
	DrainTimeout    time.Duration
	IdleTimeout     time.Duration
	MaxConns        int64
	DialRetries     int
	DialBackoffBase time.Duration
	DialBackoffMax  time.Duration
	HealthAddr      string
	LogFormat       string
	ManualMode      bool
	PortRange       string
	Config          string
	Reset           bool
}

// Merge applies the precedence chain: flags > env > yaml > defaults.
// Returns a fully-populated Config.
func Merge(yamlCfg PartialConfig, flags FlagSet) (Config, error) {
	// Reject auth key in YAML — it must come from env or --auth-key-file.
	if yamlCfg.AuthKey != "" {
		return Config{}, fmt.Errorf("auth key in YAML config is not allowed; use TS_AUTHKEY env var or --auth-key-file flag")
	}

	// Build config from lowest precedence to highest:
	// defaults → YAML → env → flags
	cfg := defaults()
	applyYAML(&cfg, yamlCfg)
	applyEnv(&cfg)
	applyFlags(&cfg, flags)

	// Validate required fields.
	if cfg.Target == "" {
		return Config{}, fmt.Errorf("target is required (provide --target flag, TS_TARGET env var, or YAML config)")
	}
	if cfg.AuthKey == "" {
		return Config{}, fmt.Errorf("auth key is required (provide TS_AUTHKEY env var or --auth-key-file flag)")
	}
	if !strings.HasPrefix(cfg.AuthKey, "tskey-") && !strings.HasPrefix(cfg.AuthKey, "hskey-") {
		return Config{}, fmt.Errorf("auth key invalid format (must start with tskey- or hskey-)")
	}

	// Validate target format.
	if err := validateTarget(cfg.Target); err != nil {
		return Config{}, err
	}

	// Validate flag-level inputs before applying.
	if flags.DialRetries < 0 {
		return Config{}, fmt.Errorf("dial retries must be non-negative (0 disables retries), got: %d", flags.DialRetries)
	}

	// Validate env-level inputs before applying.
	if v := os.Getenv("TS_DIAL_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n < 0 {
			return Config{}, fmt.Errorf("dial retries must be non-negative (0 disables retries), got: %d", n)
		}
	}

	// Validate dial retries is non-negative after all sources are merged.
	if cfg.DialRetries < 0 {
		return Config{}, fmt.Errorf("dial retries must be non-negative (0 disables retries), got: %d", cfg.DialRetries)
	}

	// Apply auto-instance mode.
	if !flags.ManualMode {
		applyAutoInstance(&cfg, flags)
	}

	return cfg, nil
}

// defaults returns a Config with built-in defaults.
func defaults() Config {
	return Config{
		ConnectTimeout:  defaultTimeout,
		DialTimeout:     defaultDialTimeout,
		DrainTimeout:    defaultDrainTimeout,
		DialRetries:     defaultDialRetries,
		DialBackoffBase: defaultDialBackoffBase,
		DialBackoffMax:  defaultDialBackoffMax,
		MaxConnections:  defaultMaxConnections,
		LogFormat:       "text",
		AutoInstance:    true,
	}
}

func applyYAML(cfg *Config, yamlCfg PartialConfig) {
	// String fields — apply if non-empty.
	applyStringFields(cfg, yamlCfg.Target, yamlCfg.Hostname, yamlCfg.ControlURL,
		yamlCfg.StateDir, yamlCfg.HealthAddr, yamlCfg.LogFormat)

	// Duration fields — apply if positive.
	applyDurationFields(cfg, yamlCfg.Timeout, yamlCfg.DialTimeout,
		yamlCfg.DrainTimeout, yamlCfg.IdleTimeout, yamlCfg.DialBackoffBase, yamlCfg.DialBackoffMax)

	// Int fields.
	if yamlCfg.DialRetries > 0 {
		cfg.DialRetries = yamlCfg.DialRetries
	}
	if yamlCfg.MaxConnections > 0 {
		cfg.MaxConnections = yamlCfg.MaxConnections
	}
	if yamlCfg.AutoInstance != nil {
		cfg.AutoInstance = *yamlCfg.AutoInstance
	}
}

func applyStringFields(cfg *Config, target, hostname, controlURL, stateDir, healthAddr, logFormat string) {
	if target != "" {
		cfg.Target = target
	}
	if hostname != "" {
		cfg.Hostname = hostname
	}
	if controlURL != "" {
		cfg.ControlURL = controlURL
	}
	if stateDir != "" {
		cfg.StateDir = stateDir
	}
	if healthAddr != "" {
		cfg.HealthAddr = healthAddr
	}
	if logFormat != "" {
		cfg.LogFormat = logFormat
	}
}

func applyDurationFields(cfg *Config, timeout, dialTimeout, drainTimeout, idleTimeout, backoffBase, backoffMax time.Duration) {
	if timeout > 0 {
		cfg.ConnectTimeout = timeout
	}
	if dialTimeout > 0 {
		cfg.DialTimeout = dialTimeout
	}
	if drainTimeout > 0 {
		cfg.DrainTimeout = drainTimeout
	}
	if idleTimeout >= 0 {
		cfg.IdleTimeout = idleTimeout
	}
	if backoffBase > 0 {
		cfg.DialBackoffBase = backoffBase
	}
	if backoffMax > 0 {
		cfg.DialBackoffMax = backoffMax
	}
}

func applyEnv(cfg *Config) {
	// String fields.
	applyEnvString(&cfg.Target, "TS_TARGET")
	applyEnvString(&cfg.AuthKey, "TS_AUTHKEY")
	applyEnvString(&cfg.LocalAddr, "TS_LOCAL_ADDR")
	applyEnvString(&cfg.Hostname, "TS_HOSTNAME")
	applyEnvString(&cfg.StateDir, "TS_STATE_DIR")
	applyEnvString(&cfg.ControlURL, "TS_CONTROL_URL")
	applyEnvString(&cfg.HealthAddr, "TS_HEALTH_ADDR")
	applyEnvString(&cfg.LogFormat, "TS_LOG_FORMAT")

	// Duration fields.
	applyEnvDuration(&cfg.ConnectTimeout, "TS_TIMEOUT", nil)
	applyEnvDuration(&cfg.DrainTimeout, "TS_DRAIN_TIMEOUT", nil)
	applyEnvDuration(&cfg.IdleTimeout, "TS_IDLE_TIMEOUT", nonNegativeDuration)
	applyEnvDuration(&cfg.DialTimeout, "TS_DIAL_TIMEOUT", positiveDuration)
	applyEnvDuration(&cfg.DialBackoffBase, "TS_DIAL_BACKOFF_BASE", nonNegativeDuration)
	applyEnvDuration(&cfg.DialBackoffMax, "TS_DIAL_BACKOFF_MAX", nonNegativeDuration)

	// Integer fields.
	applyEnvInt(&cfg.DialRetries, "TS_DIAL_RETRIES", nonNegativeInt)
	applyEnvInt64(&cfg.MaxConnections, "TS_MAX_CONNECTIONS", positiveInt64)
}

func applyEnvString(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

func applyEnvDuration(dst *time.Duration, key string, validate func(time.Duration) bool) {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: %s=%q is not a valid duration: %v\n", key, v, err)
		} else if validate != nil && !validate(d) {
			fmt.Fprintf(os.Stderr, "WARNING: %s=%v failed validation\n", key, d)
		} else {
			*dst = d
		}
	}
}

func applyEnvInt(dst *int, key string, validate func(int) bool) {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: %s=%q is not a valid integer: %v\n", key, v, err)
		} else if validate != nil && !validate(n) {
			fmt.Fprintf(os.Stderr, "WARNING: %s=%d failed validation\n", key, n)
		} else {
			*dst = n
		}
	}
}

func applyEnvInt64(dst *int64, key string, validate func(int64) bool) {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: %s=%q is not a valid integer: %v\n", key, v, err)
		} else if validate != nil && !validate(n) {
			fmt.Fprintf(os.Stderr, "WARNING: %s=%d failed validation\n", key, n)
		} else {
			*dst = n
		}
	}
}

func nonNegativeDuration(d time.Duration) bool { return d >= 0 }
func positiveDuration(d time.Duration) bool    { return d > 0 }
func nonNegativeInt(n int) bool               { return n >= 0 }
func positiveInt64(n int64) bool              { return n > 0 }

func applyFlags(cfg *Config, flags FlagSet) {
	// String fields.
	applyFlagString(&cfg.Target, flags.Target)
	applyFlagString(&cfg.AuthKey, flags.AuthKey)
	applyFlagString(&cfg.LocalAddr, flags.LocalAddr)
	applyFlagString(&cfg.Hostname, flags.Hostname)
	applyFlagString(&cfg.StateDir, flags.StateDir)
	applyFlagString(&cfg.ControlURL, flags.ControlURL)
	applyFlagString(&cfg.HealthAddr, flags.HealthAddr)
	applyFlagString(&cfg.LogFormat, flags.LogFormat)

	// Duration fields — apply only if positive.
	if flags.Timeout > 0 {
		cfg.ConnectTimeout = flags.Timeout
	}
	if flags.DialTimeout > 0 {
		cfg.DialTimeout = flags.DialTimeout
	}
	if flags.DrainTimeout > 0 {
		cfg.DrainTimeout = flags.DrainTimeout
	}
	if flags.IdleTimeout >= 0 {
		cfg.IdleTimeout = flags.IdleTimeout
	}
	if flags.DialBackoffBase > 0 {
		cfg.DialBackoffBase = flags.DialBackoffBase
	}
	if flags.DialBackoffMax > 0 {
		cfg.DialBackoffMax = flags.DialBackoffMax
	}

	// Integer fields.
	if flags.MaxConns > 0 {
		cfg.MaxConnections = flags.MaxConns
	}
	if flags.DialRetries >= 0 {
		cfg.DialRetries = flags.DialRetries
	}
}

func applyFlagString(dst *string, val string) {
	if val != "" {
		*dst = val
	}
}

func applyAutoInstance(cfg *Config, flags FlagSet) {
	// Check if auto-instance should be disabled.
	if flags.PortRange != "" || flags.Instance != "" {
		// User provided instance-specific flags → enable auto mode.
		cfg.AutoInstance = true
	}

	if !cfg.AutoInstance {
		return
	}

	instanceName := flags.Instance
	portRange := flags.PortRange

	if cfg.LocalAddr == "" && portRange != "" {
		localAddr, err := deriveAutoLocalAddr(cfg.Target, instanceName, portRange)
		if err == nil {
			cfg.LocalAddr = localAddr
		}
	}

	if cfg.Hostname == "" && instanceName != "" {
		cfg.Hostname = deriveAutoHostname(cfg.Target, instanceName)
	}

	if cfg.StateDir == "" && cfg.Hostname != "" {
		cfg.StateDir = defaultStateDir
		cfg.EphemeralState = true
	}
}

// validateTarget checks the target format.
func validateTarget(target string) error {
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return fmt.Errorf("target invalid format: %w", err)
	}
	if host == "" {
		return fmt.Errorf("target: host cannot be empty")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("target: invalid port %q", portStr)
	}
	return nil
}

// LoadYAMLConfig reads and parses a YAML config file.
// Returns an empty PartialConfig if the file does not exist (not an error).
func LoadYAMLConfig(path string) (PartialConfig, error) {
	if path == "" {
		return PartialConfig{}, nil
	}

	// #nosec G304 -- path is from --config flag (user-controlled) and validated non-empty above.
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PartialConfig{}, nil
		}
		return PartialConfig{}, fmt.Errorf("read YAML config: %w", err)
	}

	// Warn on world-readable permissions (Unix only).
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err == nil && info.Mode().Perm()&0077 != 0 {
			fmt.Fprintf(os.Stderr, "WARNING: YAML config file has loose permissions (%04o); consider chmod 600 %s\n",
				info.Mode().Perm(), path)
		}
	}

	var cfg PartialConfig
	if err := decodeYAML(data, &cfg); err != nil {
		return PartialConfig{}, err
	}

	return cfg, nil
}
