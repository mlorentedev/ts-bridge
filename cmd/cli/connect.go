package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"ts-bridge/internal/config"
	"ts-bridge/internal/config/envfile"
)

const windowsOS = "windows"

// newConnectCmd constructs the "ts-bridge connect" subcommand.
func newConnectCmd() *cobra.Command { //nolint:funlen // baseline #629
	cmd := &cobra.Command{
		Use:   "connect [flags]",
		Short: "Start the TCP bridge",
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

	// Required-like flags (one of: flag, env, or YAML).
	cmd.Flags().String("target", "", "Target address HOST:PORT (overrides TS_TARGET)")
	cmd.Flags().String("socks5", "", "Loopback SOCKS5 listener address for dynamic mesh targets")
	cmd.Flags().StringArray("route", nil, "Allow-listed SOCKS5 route SOURCE_HOST:PORT=MESH_HOST:PORT (repeatable)")
	cmd.Flags().String("auth-key", "", "Auth key (overrides TS_AUTHKEY) — WARNING: visible in process list")
	cmd.Flags().String("auth-key-file", "", "Read auth key from file (secure alternative to --auth-key)")

	// Instance / auto-mode flags.
	cmd.Flags().String("instance", "", "Instance name for auto-mode")
	cmd.Flags().String("local-addr", "", "Local bind address")
	cmd.Flags().String("hostname", "", "Tailscale hostname")
	cmd.Flags().String("state-dir", "", "State directory")
	cmd.Flags().String("control-url", "", "Custom control plane URL")
	cmd.Flags().String("bootstrap-ssh", "", "Reach the control plane through OpenSSH user@host[:port]")
	cmd.Flags().String("bootstrap-socks-addr", "", "Loopback SOCKS address used by SSH bootstrap")

	// Timeouts.
	cmd.Flags().Duration("timeout", 0, "Connect timeout for tsnet init")
	cmd.Flags().Duration("dial-timeout", 0, "Per-dial timeout")
	cmd.Flags().Duration("idle-timeout", 0, "Idle connection timeout (0 = disabled)")
	cmd.Flags().Duration("drain-timeout", 0, "Graceful drain timeout")

	// Limits.
	cmd.Flags().Int64("max-conns", 0, "Max concurrent connections")

	// Dial retry.
	cmd.Flags().Int("dial-retries", 0, "Dial retry count")
	cmd.Flags().Duration("dial-backoff-base", 0, "Dial backoff base")
	cmd.Flags().Duration("dial-backoff-max", 0, "Dial backoff max")

	// Health.
	cmd.Flags().String("health-addr", "", "Health server address")

	// Logging.
	cmd.Flags().String("log-format", "", "Log format (text|json)")

	// Mode.
	cmd.Flags().Bool("manual-mode", false, "Disable auto-instance mode")
	cmd.Flags().String("port-range", "", "Auto port range (e.g. 33389-34388)")

	// Misc.
	cmd.Flags().Bool("reset", false, "Reset state (manual mode only; no-op in auto mode)")
	cmd.Flags().String("config", "", "Path to YAML config file (default: none)")
	cmd.Flags().String("profile", "", "Named profile defaults (flags, environment, and YAML take precedence)")
	cmd.Flags().Bool("quiet", false, "Suppress the startup banner; the structured READY/ERROR lines still print")
	return cmd
}

func runConnect(cmd *cobra.Command, args []string) error { //nolint:funlen,gocyclo // baseline #629
	// Collect CLI flag values.
	flags := collectFlags(cmd)

	// Detect if --auth-key was explicitly provided (before we potentially overwrite it).
	authKeyFlagProvided := cmd.Flags().Changed("auth-key")

	// Resolve auth key source before Merge validation.
	// --auth-key-file takes precedence over --auth-key.
	if flags.AuthKeyFile != "" {
		key, err := readAuthKeyFile(flags.AuthKeyFile)
		if err != nil {
			// readAuthKeyFile names the file in every error it returns (#401).
			return err
		}
		flags.AuthKey = key
	}

	// Warn about --auth-key visibility if it was explicitly provided
	// (even when --auth-key-file also provided and takes precedence).
	if authKeyFlagProvided {
		fmt.Fprintln(os.Stderr, "WARNING: --auth-key is visible in the process list; use --auth-key-file instead")
	}

	// Auto-load .env from CWD if it exists.
	// This mirrors docker-compose, webpack, rails conventions: the .env
	// file created by "ts-bridge init" is automatically picked up by
	// "ts-bridge connect" so the user never has to manually source it.
	//
	// Precedence is preserved: CLI flags > explicit env vars > .env > YAML > defaults.
	// Load() writes into os.Setenv, and the merge chain reads from os.Getenv,
	// so explicit env vars set before this call still win over .env values.
	if err := envfile.Load(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}

	// Load YAML config (optional, not an error if missing).
	yamlCfg, err := config.LoadYAMLConfig(flags.Config)
	if err != nil {
		return fmt.Errorf("load YAML config: %w", err)
	}

	// Profiles are the lowest configuration source above defaults. Load the
	// selected profile before Merge validates required and control-plane fields.
	profileName, _ := cmd.Flags().GetString("profile")
	profileTarget, err := applyConnectProfile(&yamlCfg, &flags, profileName)
	if err != nil {
		return err
	}

	// Merge: flags > env > yaml > profile > defaults.
	cfg, err := config.Merge(yamlCfg, flags)
	if err != nil {
		return withAuthSetHint(err, profileName)
	}

	// --quiet is a console-output concern (like --profile, resolved
	// post-Merge — no env/YAML precedence needed).
	cfg.Quiet, _ = cmd.Flags().GetBool("quiet")

	// --reset is no-op in auto mode (ephemeral state by design).

	release, err := lockCommandProfile(cmd)
	if err != nil {
		return err
	}
	defer release()

	// Initialize logger.
	if LoggerInit != nil {
		LoggerInit(cfg)
	}
	warnConnectTargetOverride(profileName, profileTarget, cfg.Target, flags.Target)

	// Run the bridge.
	if Runner != nil {
		return Runner(cfg)
	}
	return fmt.Errorf("bridge runner not initialized (call main.go first)")
}

// withAuthSetHint points a missing-credential error at the managed path when a
// profile is selected, since the profile is what auth set attaches a key to.
func withAuthSetHint(err error, profileName string) error {
	if profileName == "" || !errors.Is(err, config.ErrAuthKeyRequired) {
		return err
	}
	return fmt.Errorf("%w\nstore one for the profile: ts-bridge auth set --profile %s", err, profileName)
}

func applyConnectProfile(yamlCfg *config.PartialConfig, flags *config.FlagSet, profileName string) (string, error) {
	selectedProfile, err := loadCommandProfile(profileName)
	if err != nil {
		return "", err
	}
	// Bind the credential before the profile fills the YAML layer, which would
	// hide which layer set the control URL.
	if flags.AuthKey == "" && flags.AuthKeyFile == "" && os.Getenv("TS_AUTHKEY") == "" {
		flags.ProfileAuthKey, err = loadBoundProfileCredential(profileName, selectedProfile, *flags, *yamlCfg)
		if err != nil {
			return selectedProfile.Target, err
		}
	}
	if yamlCfg.Target == "" {
		yamlCfg.Target = selectedProfile.Target
	}
	if yamlCfg.ControlURL == "" {
		yamlCfg.ControlURL = selectedProfile.ControlURL
	}
	applyProfileRuntime(yamlCfg, selectedProfile)
	return selectedProfile.Target, nil
}

// sameConnectTarget reports whether two targets name one destination: host names
// are case-insensitive, ports and IPv6 zone identifiers are compared exactly (#508).
func sameConnectTarget(a, b string) bool {
	hostA, portA, errA := net.SplitHostPort(a)
	hostB, portB, errB := net.SplitHostPort(b)
	if errA != nil || errB != nil {
		// No parseable port on one side: compare the whole targets as hosts.
		return sameConnectHost(a, b)
	}
	return sameConnectHost(hostA, hostB) && portA == portB
}

// sameConnectHost folds the case of the address but not of an IPv6 zone, which
// names an interface and is case-sensitive.
func sameConnectHost(a, b string) bool {
	addrA, zoneA, _ := strings.Cut(a, "%")
	addrB, zoneB, _ := strings.Cut(b, "%")
	return strings.EqualFold(addrA, addrB) && zoneA == zoneB
}

func warnConnectTargetOverride(profileName, profileTarget, target, flagTarget string) {
	if profileName == "" || profileTarget == "" || target == "" || sameConnectTarget(target, profileTarget) {
		return
	}
	source := "YAML"
	if os.Getenv("TS_TARGET") != "" {
		source = "TS_TARGET"
	}
	if flagTarget != "" {
		source = "--target"
	}
	selectedLogger := logger
	if selectedLogger == nil {
		selectedLogger = slog.Default()
	}
	selectedLogger.Warn("connection target overrides selected profile",
		"profile", profileName, "source", source, "target", target)
}

// collectFlags reads all CLI flag values into a FlagSet.
func collectFlags(cmd *cobra.Command) config.FlagSet {
	var fs config.FlagSet

	fs.Target, _ = cmd.Flags().GetString("target")
	fs.SOCKS5Addr, _ = cmd.Flags().GetString("socks5")
	fs.SOCKS5Routes, _ = cmd.Flags().GetStringArray("route")
	fs.AuthKey, _ = cmd.Flags().GetString("auth-key")
	fs.AuthKeyFile, _ = cmd.Flags().GetString("auth-key-file")
	fs.Instance, _ = cmd.Flags().GetString("instance")
	fs.LocalAddr, _ = cmd.Flags().GetString("local-addr")
	fs.Hostname, _ = cmd.Flags().GetString("hostname")
	fs.StateDir, _ = cmd.Flags().GetString("state-dir")
	fs.ControlURL, _ = cmd.Flags().GetString("control-url")
	fs.BootstrapSSH, _ = cmd.Flags().GetString("bootstrap-ssh")
	fs.BootstrapSOCKSAddr, _ = cmd.Flags().GetString("bootstrap-socks-addr")
	fs.HealthAddr, _ = cmd.Flags().GetString("health-addr")
	fs.LogFormat, _ = cmd.Flags().GetString("log-format")
	fs.PortRange, _ = cmd.Flags().GetString("port-range")
	fs.Config, _ = cmd.Flags().GetString("config")

	fs.Timeout, _ = cmd.Flags().GetDuration("timeout")
	fs.DialTimeout, _ = cmd.Flags().GetDuration("dial-timeout")
	fs.DrainTimeout, _ = cmd.Flags().GetDuration("drain-timeout")
	fs.DialBackoffBase, _ = cmd.Flags().GetDuration("dial-backoff-base")
	fs.DialBackoffMax, _ = cmd.Flags().GetDuration("dial-backoff-max")

	fs.MaxConns, _ = cmd.Flags().GetInt64("max-conns")

	collectOptionalNumericFlags(cmd, &fs)
	fs.Verbose = collectVerboseFlag(cmd)

	fs.ManualMode, _ = cmd.Flags().GetBool("manual-mode")
	fs.Reset, _ = cmd.Flags().GetBool("reset")

	// Validate: target is required (from flag, env, or YAML).
	// Validation happens in Merge().

	return fs
}

// collectOptionalNumericFlags records --idle-timeout and --dial-retries only
// when the user actually passed them.
//
// 0 is a meaningful value for both (it disables the feature), so cobra's own
// default of 0 is indistinguishable from "not supplied" by value alone.
// Reading them unconditionally sent that 0 into Merge, where it overwrote
// whatever env or YAML had provided — and the built-in default of 3 retries
// along with it. (#282)
func collectOptionalNumericFlags(cmd *cobra.Command, fs *config.FlagSet) {
	if cmd.Flags().Changed("idle-timeout") {
		v, _ := cmd.Flags().GetDuration("idle-timeout")
		fs.IdleTimeout = &v
	}
	if cmd.Flags().Changed("dial-retries") {
		v, _ := cmd.Flags().GetInt("dial-retries")
		fs.DialRetries = &v
	}
}

// collectVerboseFlag returns the persistent --verbose flag only when the user
// passed it, so an unset flag does not override TS_VERBOSE and an explicit
// --verbose=false does.
func collectVerboseFlag(cmd *cobra.Command) *bool {
	if !cmd.Flags().Changed("verbose") {
		return nil
	}
	v, _ := cmd.Flags().GetBool("verbose")
	return &v
}

// readAuthKeyFile reads an auth key from a file with permission checks.
func readAuthKeyFile(path string) (string, error) {
	// Resolve to absolute path for permission checks.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve auth key file path: %w", err)
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
	if strings.ContainsAny(key, "\r\n") {
		return "", fmt.Errorf("auth key file contains embedded line break: %s", absPath)
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
