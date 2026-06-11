package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"ts-bridge/internal/config"
)

const windowsOS = "windows"

// connectCmd is the "ts-bridge connect" subcommand.
var connectCmd = &cobra.Command{
	Use:   "connect [flags]",
	Short: "Start the TCP bridge (replaces run.sh / run.ps1)",
	Long: `Start the TCP bridge that forwards local connections to a remote
target over the Tailscale mesh network.

All flags are optional — defaults come from env vars, YAML config, or
built-in defaults. Precedence (highest to lowest):

  1. CLI flags
  2. Environment variables (TS_*)
  3. YAML config file (--config)
  4. Built-in defaults

Examples:
  ts-bridge connect --target 100.64.0.1:3389
  ts-bridge connect --auth-key-file /run/secrets/authkey
  ts-bridge connect --config ts-bridge.yaml --manual-mode
  ts-bridge connect --reset`,
	RunE: runConnect,
}

func init() {
	// Required-like flags (one of: flag, env, or YAML).
	connectCmd.Flags().String("target", "", "Target address HOST:PORT (overrides TS_TARGET)")
	connectCmd.Flags().String("auth-key", "", "Auth key (overrides TS_AUTHKEY) — WARNING: visible in process list")
	connectCmd.Flags().String("auth-key-file", "", "Read auth key from file (secure alternative to --auth-key)")

	// Instance / auto-mode flags.
	connectCmd.Flags().String("instance", "", "Instance name for auto-mode")
	connectCmd.Flags().String("local-addr", "", "Local bind address")
	connectCmd.Flags().String("hostname", "", "Tailscale hostname")
	connectCmd.Flags().String("state-dir", "", "State directory")
	connectCmd.Flags().String("control-url", "", "Custom control plane URL")

	// Timeouts.
	connectCmd.Flags().Duration("timeout", 0, "Connect timeout for tsnet init")
	connectCmd.Flags().Duration("dial-timeout", 0, "Per-dial timeout")
	connectCmd.Flags().Duration("idle-timeout", 0, "Idle connection timeout (0 = disabled)")
	connectCmd.Flags().Duration("drain-timeout", 0, "Graceful drain timeout")

	// Limits.
	connectCmd.Flags().Int64("max-conns", 0, "Max concurrent connections")

	// Dial retry.
	connectCmd.Flags().Int("dial-retries", 0, "Dial retry count")
	connectCmd.Flags().Duration("dial-backoff-base", 0, "Dial backoff base")
	connectCmd.Flags().Duration("dial-backoff-max", 0, "Dial backoff max")

	// Health.
	connectCmd.Flags().String("health-addr", "", "Health server address")

	// Logging.
	connectCmd.Flags().String("log-format", "", "Log format (text|json)")

	// Mode.
	connectCmd.Flags().Bool("manual-mode", false, "Disable auto-instance mode")
	connectCmd.Flags().String("port-range", "", "Auto port range (e.g. 33389-34388)")

	// Misc.
	connectCmd.Flags().Bool("reset", false, "Reset state (manual mode only; no-op in auto mode)")
	connectCmd.Flags().String("config", "", "Path to YAML config file (default: none)")

	// Register the subcommand.
	rootCmd.AddCommand(connectCmd)
}

func runConnect(cmd *cobra.Command, args []string) error {
	// Collect CLI flag values.
	flags := collectFlags(cmd)

	// Load YAML config (optional, not an error if missing).
	yamlCfg, err := config.LoadYAMLConfig(flags.Config)
	if err != nil {
		return fmt.Errorf("load YAML config: %w", err)
	}

	// Merge: flags > env > yaml > defaults.
	cfg, err := config.Merge(yamlCfg, flags)
	if err != nil {
		return err
	}

	// Handle --auth-key-file: read key from file.
	if flags.AuthKeyFile != "" {
		key, err := readAuthKeyFile(flags.AuthKeyFile)
		if err != nil {
			return fmt.Errorf("read auth key file: %w", err)
		}
		cfg.AuthKey = key
	}

	// Handle --auth-key: log warning about process list visibility.
	if flags.AuthKey != "" {
		fmt.Fprintln(os.Stderr, "WARNING: --auth-key is visible in the process list; use --auth-key-file instead")
	}

	// --reset is no-op in auto mode (ephemeral state by design).

	// Initialize logger.
	if LoggerInit != nil {
		LoggerInit(cfg)
	}

	// Run the bridge.
	if Runner != nil {
		return Runner(cfg)
	}
	return fmt.Errorf("bridge runner not initialized (call main.go first)")
}

// collectFlags reads all CLI flag values into a FlagSet.
func collectFlags(cmd *cobra.Command) config.FlagSet {
	var fs config.FlagSet

	fs.Target, _ = cmd.Flags().GetString("target")
	fs.AuthKey, _ = cmd.Flags().GetString("auth-key")
	fs.AuthKeyFile, _ = cmd.Flags().GetString("auth-key-file")
	fs.Instance, _ = cmd.Flags().GetString("instance")
	fs.LocalAddr, _ = cmd.Flags().GetString("local-addr")
	fs.Hostname, _ = cmd.Flags().GetString("hostname")
	fs.StateDir, _ = cmd.Flags().GetString("state-dir")
	fs.ControlURL, _ = cmd.Flags().GetString("control-url")
	fs.HealthAddr, _ = cmd.Flags().GetString("health-addr")
	fs.LogFormat, _ = cmd.Flags().GetString("log-format")
	fs.PortRange, _ = cmd.Flags().GetString("port-range")
	fs.Config, _ = cmd.Flags().GetString("config")

	fs.Timeout, _ = cmd.Flags().GetDuration("timeout")
	fs.DialTimeout, _ = cmd.Flags().GetDuration("dial-timeout")
	fs.IdleTimeout, _ = cmd.Flags().GetDuration("idle-timeout")
	fs.DrainTimeout, _ = cmd.Flags().GetDuration("drain-timeout")
	fs.DialBackoffBase, _ = cmd.Flags().GetDuration("dial-backoff-base")
	fs.DialBackoffMax, _ = cmd.Flags().GetDuration("dial-backoff-max")

	fs.MaxConns, _ = cmd.Flags().GetInt64("max-conns")
	fs.DialRetries, _ = cmd.Flags().GetInt("dial-retries")

	fs.ManualMode, _ = cmd.Flags().GetBool("manual-mode")
	fs.Reset, _ = cmd.Flags().GetBool("reset")

	// Validate: target is required (from flag, env, or YAML).
	// Validation happens in Merge().

	return fs
}

// readAuthKeyFile reads an auth key from a file with permission checks.
func readAuthKeyFile(path string) (string, error) {
	// Resolve to absolute path for permission checks.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("stat auth key file: %w", err)
	}

	// Warn on world-readable permissions (Unix only).
	if runtime.GOOS != windowsOS && info.Mode().Perm()&0077 != 0 {
		fmt.Fprintf(os.Stderr, "WARNING: auth key file has loose permissions (%04o); consider chmod 600 %s\n",
			info.Mode().Perm(), absPath)
	}

	// #nosec G304 -- absPath is resolved via filepath.Abs and permissions are checked before read.
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read auth key file: %w", err)
	}

	key := string(data)
	key = trimTrailingNewline(key)
	if key == "" {
		return "", fmt.Errorf("auth key file is empty: %s", absPath)
	}

	return key, nil
}

func trimTrailingNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// Runner is the bridge start function provided by main.go.
var Runner func(config.Config) error

// LoggerInit initializes the global logger. Provided by main.go.
var LoggerInit func(config.Config)
