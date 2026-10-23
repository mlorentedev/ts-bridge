// Package cmd provides the Cobra CLI for ts-bridge.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"ts-bridge/internal/config/envfile"
	"ts-bridge/internal/host"
	"ts-bridge/internal/profile"
)

// newHostCmd constructs the "ts-bridge host" command tree.
func newHostCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host",
		Short: "Host setup and verification commands",
		Long: `Manage host machine configuration for RDP access over Tailscale.

Examples:
  ts-bridge host init
  ts-bridge host setup
  ts-bridge host setup --no-sleep
  ts-bridge host check
  ts-bridge host check --json
`,
	}
	cmd.AddCommand(newHostSetupCmd(), newHostCheckCmd(), newHostInitCmd())
	return cmd
}

func newHostSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
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
  - Enables and starts xrdp (systemctl enable --now) when it is not already
  - Allows the RDP port inbound with ufw (or iptables) when no rule allows it yet
  - Prints Tailscale IP

On macOS:
  - Not applicable — use ts-bridge connect to connect to a remote host.

Exit status:
  0  every step is ok, changed or manual (a manual step prints what is left to you)
  2  setup ran and at least one step failed
  1  setup could not run (for example, without admin privileges)
Run ts-bridge host check afterwards to confirm the host converged.

Examples:
  ts-bridge host setup
  ts-bridge host setup --no-sleep
  ts-bridge host setup --firewall-rule "MyRDPRule"
`,
		RunE: runHostSetup,
	}
	cmd.Flags().Bool("no-sleep", false, "Skip disabling sleep mode")
	cmd.Flags().String("firewall-rule", "Tailscale-RDP-Ingress", "Custom firewall rule name")
	cmd.Flags().Int("port", 0, "RDP port (default: 3389)")
	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().Bool("verbose", false, "Enable verbose (debug) logging")
	cmd.Flags().String("log-format", "", "Log format (text|json)")
	return cmd
}

func newHostCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [flags]",
		Short: "Verify host readiness (read-only)",
		Long: `Verify that the host is ready for RDP access over Tailscale.

Read-only — no administrative changes are made.

Outputs:
  - Tailscale IP address
  - RDP listening port
  - Firewall rule status

Use --json for machine-readable output.

Exit status:
  0  every step is ok
  1  the check could not run
  2  the check ran and at least one step is not ok (its fix is printed)

Examples:
  ts-bridge host check
  ts-bridge host check --json
`,
		RunE: runHostCheck,
	}
	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().Bool("verbose", false, "Enable verbose (debug) logging")
	cmd.Flags().String("log-format", "", "Log format (text|json)")
	return cmd
}

// ─── Setup ───────────────────────────────────────────────────────

func runHostSetup(cmd *cobra.Command, args []string) error {
	// macOS: not applicable.
	if hostGOOS == "darwin" {
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "Host setup is not applicable on macOS.")
		fmt.Fprintln(out, "Use the client mode (ts-bridge connect) to connect to a remote host.")
		return nil
	}

	// Admin check.
	if !hostIsElevated() {
		printElevationError(cmd.ErrOrStderr())
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

	result, err := hostSetup(cfg, logger)
	if err != nil {
		logger.Error("host setup failed", "error", err)
		return fmt.Errorf("host setup failed: %w", err)
	}

	// On Windows the RDP listening port is read from the registry, not set by
	// ts-bridge; the firewall opens the actual (registry) port. If the user
	// explicitly requested a different --port, say so rather than leaving the
	// flag silently ineffective.
	if hostGOOS == windowsOS && cmd.Flags().Changed("port") && cfg.Port != result.RDPPort {
		printWarn(cmd.ErrOrStderr(), fmt.Sprintf(
			"--port %d not applied: Windows RDP listens on port %d (set in the registry). "+
				"Change the RDP port in Windows, then re-run host setup.",
			cfg.Port, result.RDPPort))
	}

	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		if err := printSetupJSON(cmd.OutOrStdout(), result); err != nil {
			return err
		}
		return setupConvergence(result.Steps)
	}

	// Print step-by-step results.
	for i, step := range result.Steps {
		printStep(cmd.OutOrStdout(), i+1, len(result.Steps), step.Name)
		if step.Success {
			printOk(cmd.OutOrStdout(), step.Message)
		} else {
			printWarn(cmd.ErrOrStderr(), step.Message)
		}
	}

	// Report the port that was actually configured (result.RDPPort), which on
	// Windows comes from the registry and may differ from the requested --port.
	printSetupSummary(cmd.OutOrStdout(), result.RDPPort)
	return setupConvergence(result.Steps)
}

// setupConvergence returns an ExitError with exitNotConverged when any step
// failed (ADR-017 §2). Unlike check, changed and manual exit 0: setup did its
// part, and printed the action a manual step leaves to the user.
func setupConvergence(steps []host.SetupStep) error {
	var failed []string
	for _, s := range steps {
		if s.Outcome == host.OutcomeFailed {
			failed = append(failed, s.Name)
		}
	}
	if len(failed) == 0 {
		return nil
	}
	return &ExitError{Code: exitNotConverged, Err: fmt.Errorf("host setup did not converge: %d of %d steps failed: %s",
		len(failed), len(steps), strings.Join(failed, ", "))}
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

	result, err := hostCheck(cfg, logger)
	if err != nil {
		logger.Error("host check failed", "error", err)
		return fmt.Errorf("host check failed: %w", err)
	}

	return reportCheck(cmd.OutOrStdout(), result, jsonOut)
}

// reportCheck prints the check result to w and returns its exit status as an
// error.
func reportCheck(w io.Writer, result host.CheckResult, jsonOut bool) error {
	if jsonOut {
		if err := printCheckJSON(w, result); err != nil {
			return err
		}
	} else {
		printCheckText(w, result)
	}
	return checkConvergence(result.Steps)
}

// hostCheck, hostSetup, hostIsElevated and hostGOOS are what the host commands
// run on; tests replace them to drive the commands end to end without the
// machine's real state. hostGOOS steers only the command layer: host.Setup and
// host.Check read runtime.GOOS themselves, so a test that changes hostGOOS
// must also replace hostSetup or hostCheck.
var (
	hostGOOS       = runtime.GOOS
	hostCheck      = host.Check
	hostSetup      = host.Setup
	hostIsElevated = host.IsElevated
)

// exitNotConverged is the exit status of a host command that ran but did not
// find (check) or reach (setup) the declared state, as `terraform plan
// -detailed-exitcode` uses 2 for "changes present".
const exitNotConverged = 2

// checkConvergence returns an ExitError with exitNotConverged unless every
// step is ok (ADR-017 §2). In check, changed means setup would change it.
func checkConvergence(steps []host.SetupStep) error {
	var pending []string
	for _, s := range steps {
		if s.Outcome != host.OutcomeOK {
			pending = append(pending, fmt.Sprintf("%s (%s)", s.Name, s.Outcome))
		}
	}
	if len(pending) == 0 {
		return nil
	}
	return &ExitError{Code: exitNotConverged, Err: fmt.Errorf("host not converged: %d of %d steps not ok: %s",
		len(pending), len(steps), strings.Join(pending, ", "))}
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

func printStep(w io.Writer, num, total int, msg string) {
	fmt.Fprintf(w, "  [%d/%d] %s\n", num, total, msg)
}

func printOk(w io.Writer, msg string) {
	fmt.Fprintf(w, "       %s\n", msg)
}

func printWarn(w io.Writer, msg string) {
	fmt.Fprintf(w, "       WARNING: %s\n", msg)
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

func printElevationError(w io.Writer) {
	switch runtime.GOOS {
	case windowsOS:
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "ERROR: This operation requires Administrator privileges.")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "To run as Administrator:")
		fmt.Fprintln(w, "  1. Open PowerShell as Administrator (right-click → Run as Administrator)")
		fmt.Fprintln(w, "  2. Re-run: ts-bridge host setup")
		fmt.Fprintln(w, "")
	case "linux":
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "ERROR: This operation requires root privileges.")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "To run as root:")
		fmt.Fprintln(w, "  sudo ts-bridge host setup")
		fmt.Fprintln(w, "")
	}
}

func printSetupSummary(w io.Writer, rdpPort int) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  ---------------------------------------")
	fmt.Fprintln(w, "  HOST READY")
	fmt.Fprintln(w, "  ---------------------------------------")

	tsIP := host.TailscaleIP()
	if tsIP != "" {
		fmt.Fprintf(w, "  Tailscale IP: %s\n", tsIP)
	}
	fmt.Fprintf(w, "  RDP Port:     %d\n", rdpPort)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Client .env config:")
	fmt.Fprintf(w, "  TS_TARGET=%s:%d\n", tsIP, rdpPort)
	if desc := buildDescriptorLine(tsIP, rdpPort, ""); desc != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Shareable descriptor:")
		fmt.Fprintf(w, "  %s\n", desc)
		fmt.Fprintf(w, "  (import with: ts-bridge import <name> %q)\n", desc)
	}
	fmt.Fprintln(w)
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
	Outcome string `json:"outcome"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Fix     string `json:"fix"`
}

// serviceJSON is one entry of check's services[] (ADR-017 §3). Active is null
// until a step reads the service state.
type serviceJSON struct {
	Kind       string `json:"kind"`
	Port       int    `json:"port"`
	Enabled    bool   `json:"enabled"`
	Active     *bool  `json:"active"`
	FirewallOK bool   `json:"firewall_ok"`
}

func stepsJSON(steps []host.SetupStep) []setupStepJSON {
	out := make([]setupStepJSON, 0, len(steps))
	for _, s := range steps {
		out = append(out, setupStepJSON{Name: s.Name, Outcome: string(s.Outcome), Success: s.Success, Message: s.Message, Fix: s.Fix})
	}
	return out
}

func setupJSON(result host.SetupResult) ([]byte, error) {
	output := setupJSONOutput{
		RDPPort:  result.RDPPort,
		Steps:    stepsJSON(result.Steps),
		Warnings: []string{},
		Errors:   []string{},
	}
	for _, step := range result.Steps {
		if !step.Success {
			output.Warnings = append(output.Warnings, step.Message)
		}
	}
	return json.MarshalIndent(output, "", "  ")
}

func printSetupJSON(w io.Writer, result host.SetupResult) error {
	data, err := setupJSON(result)
	if err != nil {
		return fmt.Errorf("marshal setup result: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

func printCheckText(w io.Writer, r host.CheckResult) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  HOST CHECK")
	fmt.Fprintln(w, "  ---------------------------------------")
	fmt.Fprintf(w, "  Platform:      %s\n", runtime.GOOS)
	fmt.Fprintf(w, "  Tailscale:     %s\n", statusStr(r.TailscaleUp))
	fmt.Fprintf(w, "  Tailscale IP:  %s\n", r.TailscaleIP)
	fmt.Fprintf(w, "  RDP:           %s (port %d)\n", statusStr(r.RDPEnabled), r.RDPPort)
	fmt.Fprintf(w, "  Firewall:      %s\n", statusStr(r.FirewallOK))
	for _, step := range r.Steps {
		if step.Outcome != host.OutcomeOK {
			fmt.Fprintf(w, "  ! %s (%s): %s\n", step.Name, step.Outcome, step.Fix)
		}
	}
	if desc := buildDescriptorLine(r.TailscaleIP, r.RDPPort, ""); desc != "" {
		fmt.Fprintf(w, "  Descriptor:    %s\n", desc)
	}
	fmt.Fprintln(w, "  ---------------------------------------")
	fmt.Fprintln(w)
}

func checkJSON(r host.CheckResult) ([]byte, error) {
	type jsonOutput struct {
		TailscaleIP string          `json:"tailscale_ip"`
		RDPPort     int             `json:"rdp_port"`
		RDPEnabled  bool            `json:"rdp_enabled"`
		FirewallOK  bool            `json:"firewall_ok"`
		TailscaleUp bool            `json:"tailscale_up"`
		Platform    string          `json:"platform"`
		Descriptor  string          `json:"descriptor,omitempty"`
		Services    []serviceJSON   `json:"services"`
		Steps       []setupStepJSON `json:"steps"`
	}

	out := jsonOutput{
		TailscaleIP: r.TailscaleIP,
		RDPPort:     r.RDPPort,
		RDPEnabled:  r.RDPEnabled,
		FirewallOK:  r.FirewallOK,
		TailscaleUp: r.TailscaleUp,
		Platform:    runtime.GOOS,
		Descriptor:  buildDescriptorLine(r.TailscaleIP, r.RDPPort, ""),
		Services:    make([]serviceJSON, 0, len(r.Services)),
		Steps:       stepsJSON(r.Steps),
	}
	for _, svc := range r.Services {
		out.Services = append(out.Services, serviceJSON(svc))
	}
	return json.MarshalIndent(out, "", "  ")
}

func printCheckJSON(w io.Writer, r host.CheckResult) error {
	data, err := checkJSON(r)
	if err != nil {
		return fmt.Errorf("marshal check result: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

func statusStr(ok bool) string {
	if ok {
		return "✓ OK"
	}
	return "✗ NOT OK"
}
