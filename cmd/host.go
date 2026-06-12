// Package cmd provides the Cobra CLI for ts-bridge.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"ts-bridge/internal/host"
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
	checkCmd.Flags().Bool("json", false, "Output in JSON format")

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

	fmt.Println()
	fmt.Println("  HOST SETUP")
	fmt.Println("  ---------------------------------------")

	noSleep, _ := cmd.Flags().GetBool("no-sleep")
	firewallRule, _ := cmd.Flags().GetString("firewall-rule")
	if firewallRule == "" {
		firewallRule = "Tailscale-RDP-Ingress"
	}

	result, err := host.Setup(host.SetupFlags{
		NoSleep:      noSleep,
		FirewallRule: firewallRule,
	})
	if err != nil {
		return fmt.Errorf("host setup failed: %w", err)
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

	printSetupSummary(result.RDPPort)
	return nil
}

// ─── Check ───────────────────────────────────────────────────────

func runHostCheck(cmd *cobra.Command, args []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")

	result, err := host.Check()
	if err != nil {
		return fmt.Errorf("host check failed: %w", err)
	}

	if jsonOut {
		return printCheckJSON(result)
	}

	printCheckText(result)
	return nil
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

// ─── Output helpers ──────────────────────────────────────────────

func printElevationError() {
	switch runtime.GOOS {
	case "windows":
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
	fmt.Println()
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
	}

	out := jsonOutput{
		TailscaleIP: r.TailscaleIP,
		RDPPort:     r.RDPPort,
		RDPEnabled:  r.RDPEnabled,
		FirewallOK:  r.FirewallOK,
		TailscaleUp: r.TailscaleUp,
		Platform:    runtime.GOOS,
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