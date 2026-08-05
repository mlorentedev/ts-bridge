// Package cmd provides the Cobra CLI for ts-bridge.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"ts-bridge/internal/config/envfile"
	"ts-bridge/internal/host"
	"ts-bridge/internal/profile"
)

// hostCmd is the "ts-bridge host" parent command.
var hostCmd = &cobra.Command{
	Use:   "host",
	Short: "Host setup and verification commands",
	Long: `Manage host machine configuration for RDP access over Tailscale.

Available subcommands:
  setup   Configure the host for RDP access (Windows/Linux)
  check   Verify host readiness (read-only)

Examples:
  ts-bridge host setup
  ts-bridge host setup --no-sleep
  ts-bridge host check
  ts-bridge host check --json
`,
}

// setupCmd is the "ts-bridge host setup" subcommand.
var setupCmd = &cobra.Command{
	Use:   "setup [flags]",
	Short: "Configure the host for RDP access over Tailscale",
	Long: `Configure the host machine for RDP access over the Tailscale mesh network.

On Windows:
  - Enables RDP via registry
  - Enables Tailscale unattended mode
  - Configures Windows firewall for RDP on Tailscale interface
  - Enables UPnP services
  - Sets Tailscale network profile to Private
  - Optionally disables sleep mode

On Linux:
  - Detects xrdp installation
  - Configures UFW/iptables firewall rule for RDP port
  - Prints Tailscale IP

On macOS:
  - Not applicable — use ts-bridge connect to connect to a remote host.

Examples:
  ts-bridge host setup
  ts-bridge host setup --no-sleep
  ts-bridge host setup --firewall-rule "MyRDPRule"
`,
	RunE: runHostSetup,
}

// checkCmd is the "ts-bridge host check" subcommand.
var checkCmd = &cobra.Command{
	Use:   "check [flags]",
	Short: "Verify host readiness (read-only)",
	Long: `Verify that the host is ready for RDP access over Tailscale.

Read-only — no administrative changes are made.

Outputs:
  - Tailscale IP address
  - RDP listening port
  - Firewall rule status

Use --json for machine-readable output.

Examples:
  ts-bridge host check
  ts-bridge host check --json
`,
	RunE: runHostCheck,
}

func init() {
	setupCmd.Flags().Bool("no-sleep", false, "Skip disabling sleep mode")
	setupCmd.Flags().String("firewall-rule", "Tailscale-RDP-Ingress", "Custom firewall rule name")
	setupCmd.Flags().Int("port", 0, "RDP port (default: 3389)")
	setupCmd.Flags().Bool("json", false, "Output in JSON format")
	setupCmd.Flags().Bool("verbose", false, "Enable verbose (debug) logging")
	setupCmd.Flags().String("log-format", "", "Log format (text|json)")

	checkCmd.Flags().Bool("json", false, "Output in JSON format")
	checkCmd.Flags().Bool("verbose", false, "Enable verbose (debug) logging")
	checkCmd.Flags().String("log-format", "", "Log format (text|json)")

	hostCmd.AddCommand(setupCmd)
	hostCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(hostCmd)
}

// ─── Setup ───────────────────────────────────────────────────────

func runHostSetup(cmd *cobra.Command, args []string) error {
	// macOS: not applicable.
	if runtime.GOOS == "darwin" {
		fmt.Println("Host setup is not applicable on macOS.")
		fmt.Println("Use the client mode (ts-bridge connect) to connect to a remote host.")
		return nil
	}

	// Admin check.
	if !host.IsElevated() {
		printElevationError()
		return fmt.Errorf("host setup requires elevated privileges")
	}

	// Auto-load .env from CWD so the TS_HOST_* values written by
	// "ts-bridge host init" are honored here, mirroring "ts-bridge connect".
	// Explicit env vars and CLI flags still win over .env (merge precedence).
	if err := envfile.Load(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}

	// Collect CLI flags. The *Set fields record whether each bool flag was
	// explicitly passed, so an unset flag does not clobber the env layer.
	flags := host.Flags{
		NoSleep:      mustBool(cmd, "no-sleep"),
		NoSleepSet:   cmd.Flags().Changed("no-sleep"),
		FirewallRule: mustString(cmd, "firewall-rule"),
		Port:         mustInt(cmd, "port"),
		Verbose:      mustBool(cmd, "verbose"),
		VerboseSet:   cmd.Flags().Changed("verbose"),
		LogFormat:    mustString(cmd, "log-format"),
	}

	// Merge: flags > env > defaults.
	cfg := host.Merge(flags)

	// Validate firewall rule early (before any side effects).
	if _, err := host.SanitizeFirewallRule(cfg.FirewallRule); err != nil {
		return fmt.Errorf("invalid firewall rule: %w", err)
	}

	// Build the logger from the resolved config so --log-format / --verbose
	// (and their TS_HOST_* env equivalents) actually take effect.
	logger := hostInitLogger(cfg)

	logger.Info("starting host setup",
		"firewall_rule", cfg.FirewallRule,
		"rdp_port", cfg.Port,
		"no_sleep", cfg.NoSleep,
	)

	result, err := host.Setup(cfg, logger)
	if err != nil {
		logger.Error("host setup failed", "error", err)
		return fmt.Errorf("host setup failed: %w", err)
	}

	// On Windows the RDP listening port is read from the registry, not set by
	// ts-bridge; the firewall opens the actual (registry) port. If the user
	// explicitly requested a different --port, say so rather than leaving the
	// flag silently ineffective.
	if runtime.GOOS == windowsOS && cmd.Flags().Changed("port") && cfg.Port != result.RDPPort {
		printWarn(fmt.Sprintf(
			"--port %d not applied: Windows RDP listens on port %d (set in the registry). "+
				"Change the RDP port in Windows, then re-run host setup.",
			cfg.Port, result.RDPPort))
	}

	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return printSetupJSON(result)
	}

	// Print step-by-step results.
	for i, step := range result.Steps {
		printStep(i+1, len(result.Steps), step.Name)
		if step.Success {
			printOk(step.Message)
		} else {
			printWarn(step.Message)
		}
	}

	// Report the port that was actually configured (result.RDPPort), which on
	// Windows comes from the registry and may differ from the requested --port.
	printSetupSummary(result.RDPPort)
	return nil
}

// ─── Check ───────────────────────────────────────────────────────

func runHostCheck(cmd *cobra.Command, args []string) error {
	// Auto-load .env so the check reflects the configured firewall rule / port
	// written by "ts-bridge host init", mirroring "ts-bridge connect".
	if err := envfile.Load(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}

	// check has no firewall-rule/port flags; those resolve from env + defaults.
	cfg := host.Merge(host.Flags{
		Verbose:    mustBool(cmd, "verbose"),
		VerboseSet: cmd.Flags().Changed("verbose"),
		LogFormat:  mustString(cmd, "log-format"),
	})
	logger := hostInitLogger(cfg)

	jsonOut, _ := cmd.Flags().GetBool("json")

	result, err := host.Check(cfg, logger)
	if err != nil {
		logger.Error("host check failed", "error", err)
		return fmt.Errorf("host check failed: %w", err)
	}

	if jsonOut {
		return printCheckJSON(result)
	}

	printCheckText(result)
	return nil
}

// ─── Logger initialization ──────────────────────────────────────

// hostInitLogger creates a lightweight structured logger for host commands
// from the resolved config. Host commands run independently (not after
// connect), so they need their own logger. File logging is not used for
// setup/check — only console. The output format honors cfg.LogFormat, and the
// level honors cfg.Verbose (note: the separate --json flag controls the
// command's result output, not the logger).
//
// Logs go to stderr, not stdout: the --json result is machine-readable data and
// must be the only thing on stdout, or `host check --json | jq .` fails on the
// interleaved log line. Routing rather than silencing keeps --verbose useful
// alongside --json.
func hostInitLogger(cfg host.Config) *slog.Logger {
	return newHostLogger(cfg, os.Stderr)
}

// newHostLogger builds the host command logger over an arbitrary writer. Split
// out from hostInitLogger so tests can assert format and level against a buffer
// without swapping process streams.
func newHostLogger(cfg host.Config, w io.Writer) *slog.Logger {
	level := slog.LevelInfo
	if cfg.Verbose {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	return slog.New(handler)
}

// ─── Print helpers ───────────────────────────────────────────────

func printStep(num, total int, msg string) {
	fmt.Printf("  [%d/%d] %s\n", num, total, msg)
}

func printOk(msg string) {
	fmt.Printf("       %s\n", msg)
}

func printWarn(msg string) {
	fmt.Fprintf(os.Stderr, "       WARNING: %s\n", msg)
}

// mustBool, mustString, mustInt are helper getters that ignore errors
// since Cobra always succeeds for registered flags.
func mustBool(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	return v
}

func mustString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func mustInt(cmd *cobra.Command, name string) int {
	v, _ := cmd.Flags().GetInt(name)
	return v
}

// ─── Output helpers ──────────────────────────────────────────────

func printElevationError() {
	switch runtime.GOOS {
	case windowsOS:
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "ERROR: This operation requires Administrator privileges.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "To run as Administrator:")
		fmt.Fprintln(os.Stderr, "  1. Open PowerShell as Administrator (right-click → Run as Administrator)")
		fmt.Fprintln(os.Stderr, "  2. Re-run: ts-bridge host setup")
		fmt.Fprintln(os.Stderr, "")
	case "linux":
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "ERROR: This operation requires root privileges.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "To run as root:")
		fmt.Fprintln(os.Stderr, "  sudo ts-bridge host setup")
		fmt.Fprintln(os.Stderr, "")
	}
}

func printSetupSummary(rdpPort int) {
	fmt.Println()
	fmt.Println("  ---------------------------------------")
	fmt.Println("  HOST READY")
	fmt.Println("  ---------------------------------------")

	tsIP := host.TailscaleIP()
	if tsIP != "" {
		fmt.Printf("  Tailscale IP: %s\n", tsIP)
	}
	fmt.Printf("  RDP Port:     %d\n", rdpPort)
	fmt.Println()
	fmt.Println("  Client .env config:")
	fmt.Printf("  TS_TARGET=%s:%d\n", tsIP, rdpPort)
	if desc := buildDescriptorLine(tsIP, rdpPort, ""); desc != "" {
		fmt.Println()
		fmt.Println("  Shareable descriptor:")
		fmt.Printf("  %s\n", desc)
		fmt.Printf("  (import with: ts-bridge import <name> %q)\n", desc)
	}
	fmt.Println()
}

// buildDescriptorLine returns a tsb:// descriptor string for the given host IP,
// port, and control URL, or empty string if either ip or port is missing.
// controlURL="" produces cp=saas (Tailscale SaaS).
func buildDescriptorLine(ip string, port int, controlURL string) string {
	if ip == "" || port == 0 {
		return ""
	}
	d := profile.Descriptor{Host: ip, Port: port, ControlURL: controlURL, Version: 1}
	return d.String()
}

// setupJSONOutput is the JSON structure for host setup --json output.
type setupJSONOutput struct {
	Steps    []setupStepJSON `json:"steps"`
	RDPPort  int             `json:"rdp_port"`
	Warnings []string        `json:"warnings"`
	Errors   []string        `json:"errors"`
}

type setupStepJSON struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func printSetupJSON(result host.SetupResult) error {
	output := setupJSONOutput{
		RDPPort:  result.RDPPort,
		Steps:    make([]setupStepJSON, 0, len(result.Steps)),
		Warnings: []string{},
		Errors:   []string{},
	}

	for _, step := range result.Steps {
		output.Steps = append(output.Steps, setupStepJSON{
			Name:    step.Name,
			Success: step.Success,
			Message: step.Message,
		})
		if !step.Success {
			output.Warnings = append(output.Warnings, step.Message)
		}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal setup result: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printCheckText(r host.CheckResult) {
	fmt.Println()
	fmt.Println("  HOST CHECK")
	fmt.Println("  ---------------------------------------")
	fmt.Printf("  Platform:      %s\n", runtime.GOOS)
	fmt.Printf("  Tailscale:     %s\n", statusStr(r.TailscaleUp))
	fmt.Printf("  Tailscale IP:  %s\n", r.TailscaleIP)
	fmt.Printf("  RDP:           %s (port %d)\n", statusStr(r.RDPEnabled), r.RDPPort)
	fmt.Printf("  Firewall:      %s\n", statusStr(r.FirewallOK))
	if desc := buildDescriptorLine(r.TailscaleIP, r.RDPPort, ""); desc != "" {
		fmt.Printf("  Descriptor:    %s\n", desc)
	}
	fmt.Println("  ---------------------------------------")
	fmt.Println()
}

func printCheckJSON(r host.CheckResult) error {
	type jsonOutput struct {
		TailscaleIP string `json:"tailscale_ip"`
		RDPPort     int    `json:"rdp_port"`
		RDPEnabled  bool   `json:"rdp_enabled"`
		FirewallOK  bool   `json:"firewall_ok"`
		TailscaleUp bool   `json:"tailscale_up"`
		Platform    string `json:"platform"`
		Descriptor  string `json:"descriptor,omitempty"`
	}

	out := jsonOutput{
		TailscaleIP: r.TailscaleIP,
		RDPPort:     r.RDPPort,
		RDPEnabled:  r.RDPEnabled,
		FirewallOK:  r.FirewallOK,
		TailscaleUp: r.TailscaleUp,
		Platform:    runtime.GOOS,
		Descriptor:  buildDescriptorLine(r.TailscaleIP, r.RDPPort, ""),
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal check result: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func statusStr(ok bool) string {
	if ok {
		return "✓ OK"
	}
	return "✗ NOT OK"
}
