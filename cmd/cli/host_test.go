package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	cmdpkg "ts-bridge/cmd/cli"
	"ts-bridge/internal/host"
)

// ─── host init: flag parsing tests ───────────────────────────────

// TestHostInitFlags_ValidPort verifies that valid ports are accepted.
func TestHostInitFlags_ValidPort(t *testing.T) {
	c := newTestHostInitCommand(t)
	c.SetArgs([]string{"--port", "3390"})
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetErr(&buf)

	// We can't fully run init (it tries to write a file), but we can
	// verify the command parses flags without error up to the write step.
	// Instead, we verify parseHostInitFlags behavior via the command's
	// error output when a required action fails.
	err := c.Execute()
	// The command will fail trying to write .env in a temp location,
	// but flag parsing should succeed.
	if err != nil {
		// If the error is about file writing, flag parsing was fine.
		if !strings.Contains(err.Error(), "write .env") &&
			!strings.Contains(err.Error(), "no such file") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

// TestHostInitFlags_InvalidPort verifies that invalid ports are rejected.
func TestHostInitFlags_InvalidPort(t *testing.T) {
	c := newTestHostInitCommand(t)
	c.SetArgs([]string{"--port", "99999"})
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetErr(&buf)

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for invalid port, got nil")
	}
	if !strings.Contains(err.Error(), "port must be between 1 and 65535") {
		t.Errorf("expected port validation error, got: %v", err)
	}
}

// TestHostInitFlags_InvalidFirewallRule verifies that invalid firewall rules are rejected.
func TestHostInitFlags_InvalidFirewallRule(t *testing.T) {
	c := newTestHostInitCommand(t)
	c.SetArgs([]string{"--firewall-rule", "rule with spaces"})
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetErr(&buf)

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for invalid firewall rule, got nil")
	}
	if !strings.Contains(err.Error(), "firewall rule") {
		t.Errorf("expected firewall rule error, got: %v", err)
	}
}

// TestHostInitFlags_NoSleepFlag verifies that --no-sleep is parsed correctly.
func TestHostInitFlags_NoSleepFlag(t *testing.T) {
	c := newTestHostInitCommand(t)
	c.SetArgs([]string{"--port", "3389", "--no-sleep"})
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetErr(&buf)

	err := c.Execute()
	// Will fail on file write, but flag parsing succeeds.
	if err != nil && !strings.Contains(err.Error(), "write .env") &&
		!strings.Contains(err.Error(), "no such file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── printSetupJSON tests ────────────────────────────────────────

// TestPrintSetupJSON_Serialization verifies JSON output structure.
func TestPrintSetupJSON_Serialization(t *testing.T) {
	result := host.SetupResult{
		RDPPort: 3390,
		Steps: []host.SetupStep{
			{Name: "Step 1", Success: true, Message: "OK"},
			{Name: "Step 2", Success: false, Message: "Warning msg"},
			{Name: "Step 3", Success: true, Message: "Also OK"},
		},
	}
	var out bytes.Buffer
	err := cmdpkg.PrintSetupJSONForTest(&out, result)
	if err != nil {
		t.Fatalf("printSetupJSON error: %v", err)
	}
	var parsed struct {
		Steps []json.RawMessage `json:"steps"`
	}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(parsed.Steps) != 3 {
		t.Fatalf("expected 3 steps in the output, got %d", len(parsed.Steps))
	}
}

// TestPrintSetupJSON_EmptySteps verifies JSON with no steps.
func TestPrintSetupJSON_EmptySteps(t *testing.T) {
	result := host.SetupResult{
		RDPPort: 3389,
		Steps:   []host.SetupStep{},
	}
	var out bytes.Buffer
	err := cmdpkg.PrintSetupJSONForTest(&out, result)
	if err != nil {
		t.Fatalf("printSetupJSON error: %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("output is not valid JSON:\n%s", out.String())
	}
}

// TestPrintSetupJSON_AllWarnings verifies warnings are collected.
func TestPrintSetupJSON_AllWarnings(t *testing.T) {
	result := host.SetupResult{
		RDPPort: 3389,
		Steps: []host.SetupStep{
			{Name: "Step 1", Success: false, Message: "Warning 1"},
			{Name: "Step 2", Success: false, Message: "Warning 2"},
		},
	}
	var out bytes.Buffer
	err := cmdpkg.PrintSetupJSONForTest(&out, result)
	if err != nil {
		t.Fatalf("printSetupJSON error: %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("output is not valid JSON:\n%s", out.String())
	}

	var parsed struct {
		Steps    []struct{ Success bool } `json:"steps"`
		Warnings []string                 `json:"warnings"`
	}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(parsed.Steps) != 2 {
		t.Fatalf("expected 2 steps in the output, got %d", len(parsed.Steps))
	}
	for i, step := range parsed.Steps {
		if step.Success {
			t.Errorf("step %d: expected success=false, got true", i)
		}
	}
	if want := []string{"Warning 1", "Warning 2"}; !slices.Equal(parsed.Warnings, want) {
		t.Errorf("warnings = %v, want %v", parsed.Warnings, want)
	}
}

// TestPrintCheckJSON_Serialization verifies check JSON output structure.
func TestPrintCheckJSON_Serialization(t *testing.T) {
	result := host.CheckResult{
		TailscaleIP: "100.64.0.1",
		RDPPort:     3389,
		RDPEnabled:  true,
		FirewallOK:  true,
		TailscaleUp: true,
	}

	var out bytes.Buffer
	err := cmdpkg.PrintCheckJSONForTest(&out, result)
	if err != nil {
		t.Fatalf("printCheckJSON error: %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("output is not valid JSON:\n%s", out.String())
	}

	got := decodeCheckJSON(t, out.Bytes())
	if got.TailscaleIP != "100.64.0.1" || got.RDPPort != 3389 || !got.RDPEnabled || !got.FirewallOK || !got.TailscaleUp {
		t.Errorf("decoded output = %+v, want the result's values", got)
	}
}

// TestPrintCheckJSON_Disconnected verifies check JSON when Tailscale is down.
func TestPrintCheckJSON_Disconnected(t *testing.T) {
	result := host.CheckResult{
		TailscaleIP: "",
		RDPPort:     0,
		RDPEnabled:  false,
		FirewallOK:  false,
		TailscaleUp: false,
	}

	var out bytes.Buffer
	err := cmdpkg.PrintCheckJSONForTest(&out, result)
	if err != nil {
		t.Fatalf("printCheckJSON error: %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("output is not valid JSON:\n%s", out.String())
	}

	got := decodeCheckJSON(t, out.Bytes())
	if got.TailscaleIP != "" || got.RDPEnabled || got.FirewallOK || got.TailscaleUp {
		t.Errorf("decoded output = %+v, want every field empty or false", got)
	}
}

// checkJSONFields is the part of host check --json these tests assert on.
type checkJSONFields struct {
	TailscaleIP string `json:"tailscale_ip"`
	RDPPort     int    `json:"rdp_port"`
	RDPEnabled  bool   `json:"rdp_enabled"`
	FirewallOK  bool   `json:"firewall_ok"`
	TailscaleUp bool   `json:"tailscale_up"`
}

func decodeCheckJSON(t *testing.T, data []byte) checkJSONFields {
	t.Helper()
	var got checkJSONFields
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, data)
	}
	return got
}

// ─── JSON output correctness tests ───────────────────────────────

// TestSetupJSONOutput_Structure verifies the JSON structure matches expectations.
func TestSetupJSONOutput_Structure(t *testing.T) {
	output := cmdpkg.SetupJSONOutputForTest{
		RDPPort: 3390,
		Steps: []cmdpkg.SetupStepJSONForTest{
			{Name: "Step 1", Success: true, Message: "OK"},
			{Name: "Step 2", Success: false, Message: "Warning"},
		},
		Warnings: []string{"Warning"},
		Errors:   []string{},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify top-level keys exist.
	if _, ok := parsed["steps"]; !ok {
		t.Error("expected 'steps' key in JSON output")
	}
	if _, ok := parsed["rdp_port"]; !ok {
		t.Error("expected 'rdp_port' key in JSON output")
	}
	if _, ok := parsed["warnings"]; !ok {
		t.Error("expected 'warnings' key in JSON output")
	}
	if _, ok := parsed["errors"]; !ok {
		t.Error("expected 'errors' key in JSON output")
	}

	// Verify rdp_port value.
	if port, ok := parsed["rdp_port"].(float64); !ok || port != 3390 {
		t.Errorf("expected rdp_port=3390, got %v", parsed["rdp_port"])
	}
}

// TestCheckJSONOutput_Structure verifies the check JSON structure.
func TestCheckJSONOutput_Structure(t *testing.T) {
	type jsonOutput struct {
		TailscaleIP string `json:"tailscale_ip"`
		RDPPort     int    `json:"rdp_port"`
		RDPEnabled  bool   `json:"rdp_enabled"`
		FirewallOK  bool   `json:"firewall_ok"`
		TailscaleUp bool   `json:"tailscale_up"`
		Platform    string `json:"platform"`
	}

	out := jsonOutput{
		TailscaleIP: "100.100.100.100",
		RDPPort:     3389,
		RDPEnabled:  true,
		FirewallOK:  true,
		TailscaleUp: true,
		Platform:    "windows",
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	expectedKeys := []string{"tailscale_ip", "rdp_port", "rdp_enabled", "firewall_ok", "tailscale_up", "platform"}
	for _, key := range expectedKeys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("expected key %q in check JSON output", key)
		}
	}
}

// ─── Host init command structure tests ───────────────────────────

// TestHostInitCmd_Registered verifies the init subcommand is registered.
func TestHostInitCmd_Registered(t *testing.T) {
	root := cmdpkg.NewRootCmd()

	// Verify host command exists.
	c, _, err := root.Find([]string{"host"})
	if err != nil {
		t.Fatalf("expected host command to be registered: %v", err)
	}
	if c.Name() != "host" {
		t.Errorf("expected host command name='host', got %q", c.Name())
	}

	// Verify host init command exists.
	cInit, _, err := root.Find([]string{"host", "init"})
	if err != nil {
		t.Fatalf("expected host init command to be registered: %v", err)
	}
	if cInit.Name() != "init" {
		t.Errorf("expected host init command name='init', got %q", cInit.Name())
	}
}

// TestHostInitCmd_FlagsRegistered verifies all flags are registered.
func TestHostInitCmd_FlagsRegistered(t *testing.T) {
	root := cmdpkg.NewRootCmd()
	c, _, err := root.Find([]string{"host", "init"})
	if err != nil {
		t.Fatalf("failed to find host init cmd: %v", err)
	}

	// Verify flags are accessible.
	port, _ := c.Flags().GetInt("port")
	if port != 0 {
		t.Errorf("expected default port 0, got %d", port)
	}

	rule, _ := c.Flags().GetString("firewall-rule")
	if rule != "" {
		t.Errorf("expected empty default firewall rule, got %q", rule)
	}

	noSleep, _ := c.Flags().GetBool("no-sleep")
	if noSleep {
		t.Error("expected default no-sleep false")
	}

	config, _ := c.Flags().GetString("config")
	if config != "" {
		t.Errorf("expected empty default config, got %q", config)
	}
}

// TestHostSetupCmd_FlagsRegistered verifies setup flags are registered.
func TestHostSetupCmd_FlagsRegistered(t *testing.T) {
	root := cmdpkg.NewRootCmd()
	c, _, err := root.Find([]string{"host", "setup"})
	if err != nil {
		t.Fatalf("failed to find host setup cmd: %v", err)
	}

	// Verify all flags are accessible.
	if _, err := c.Flags().GetBool("no-sleep"); err != nil {
		t.Error("expected --no-sleep flag")
	}
	if _, err := c.Flags().GetString("firewall-rule"); err != nil {
		t.Error("expected --firewall-rule flag")
	}
	if _, err := c.Flags().GetInt("port"); err != nil {
		t.Error("expected --port flag")
	}
	if _, err := c.Flags().GetBool("json"); err != nil {
		t.Error("expected --json flag")
	}
	if _, err := c.Flags().GetBool("verbose"); err != nil {
		t.Error("expected --verbose flag")
	}
	if _, err := c.Flags().GetString("log-format"); err != nil {
		t.Error("expected --log-format flag")
	}
}

// TestHostCheckCmd_FlagsRegistered verifies check flags are registered.
func TestHostCheckCmd_FlagsRegistered(t *testing.T) {
	root := cmdpkg.NewRootCmd()
	c, _, err := root.Find([]string{"host", "check"})
	if err != nil {
		t.Fatalf("failed to find host check cmd: %v", err)
	}

	if _, err := c.Flags().GetBool("json"); err != nil {
		t.Error("expected --json flag")
	}
	if _, err := c.Flags().GetBool("verbose"); err != nil {
		t.Error("expected --verbose flag")
	}
	if _, err := c.Flags().GetString("log-format"); err != nil {
		t.Error("expected --log-format flag")
	}
}

// TestHostInitFlags_BoundaryPorts verifies boundary port values.
func TestHostInitFlags_BoundaryPorts(t *testing.T) {
	tests := []struct {
		port    int
		wantErr bool
	}{
		{1, false},
		{3389, false},
		{65535, false},
		{0, false},    // 0 = "use default", valid
		{-1, true},    // negative is invalid (matches parseHostInitFlags)
		{65536, true}, // too high
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("port-%d", tt.port), func(t *testing.T) {
			c := newTestHostInitCommand(t)
			c.SetArgs([]string{"--port", fmt.Sprintf("%d", tt.port)})
			var buf bytes.Buffer
			c.SetOut(&buf)
			c.SetErr(&buf)

			err := c.Execute()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for this port")
				}
			} else {
				// Port is valid, but file write may fail.
				if err != nil && !strings.Contains(err.Error(), "write .env") &&
					!strings.Contains(err.Error(), "no such file") {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestHostInitFlags_ValidFirewallRules verifies valid firewall rule names.
func TestHostInitFlags_ValidFirewallRules(t *testing.T) {
	validRules := []string{
		"Tailscale-RDP-Ingress",
		"MyRule",
		"rule_123",
		"a",
		"ts-bridge-firewall",
	}
	for _, rule := range validRules {
		t.Run(rule, func(t *testing.T) {
			c := newTestHostInitCommand(t)
			c.SetArgs([]string{"--port", "3389", "--firewall-rule", rule})
			var buf bytes.Buffer
			c.SetOut(&buf)
			c.SetErr(&buf)

			err := c.Execute()
			if err != nil && !strings.Contains(err.Error(), "write .env") &&
				!strings.Contains(err.Error(), "no such file") {
				t.Fatalf("unexpected error for valid rule %q: %v", rule, err)
			}
		})
	}
}

// TestHostInitFlags_InvalidFirewallRules verifies invalid firewall rule names.
func TestHostInitFlags_InvalidFirewallRules(t *testing.T) {
	invalidRules := []string{
		"rule with spaces",
		"rule@domain",
		"rule#hash",
		"rule$dollar",
		"rule; drop",
		"rule$(cmd)",
	}
	for _, rule := range invalidRules {
		t.Run(rule, func(t *testing.T) {
			c := newTestHostInitCommand(t)
			c.SetArgs([]string{"--port", "3389", "--firewall-rule", rule})
			var buf bytes.Buffer
			c.SetOut(&buf)
			c.SetErr(&buf)

			err := c.Execute()
			if err == nil {
				t.Error("expected error for invalid rule")
			}
			if !strings.Contains(err.Error(), "firewall rule") {
				t.Errorf("expected firewall rule error, got: %v", err)
			}
		})
	}
}

// ─── Helpers ─────────────────────────────────────────────────────

// newTestHostInitCommand returns the real host init command, as on Windows,
// writing to a file in a fresh temp dir unless the test passes --config.
func newTestHostInitCommand(t *testing.T) *cobra.Command {
	t.Helper()
	c := cmdpkg.HostInitCmdForTest(t, "windows")
	_ = c.Flags().Lookup("config").Value.Set(filepath.Join(t.TempDir(), "host.yaml"))
	return c
}

func TestPrintCheckText_Service(t *testing.T) {
	tests := []struct {
		name    string
		result  host.CheckResult
		want    string
		notWant string
	}{
		{
			name:   "service declared",
			result: host.CheckResult{RDPPort: 3389, RDPEnabled: true},
			want:   "(port 3389)",
		},
		{
			name:    "no service declared",
			result:  host.CheckResult{},
			want:    "no service declared",
			notWant: "port 0",
		},
		{
			name: "enabled service reads as ok without elevation",
			result: host.CheckResult{RDPPort: 3389, RDPEnabled: true, Services: []host.Service{
				{Kind: host.ServiceRDP, Port: 3389, Enabled: true},
			}},
			want:    "✓ OK (port 3389)",
			notWant: "needs root/administrator",
		},
		{
			name: "declared and unreadable without elevation",
			result: host.CheckResult{Services: []host.Service{
				{Kind: host.ServiceRDP, Port: 3389, Unread: true},
			}},
			want:    "needs root/administrator",
			notWant: "no service declared",
		},
		{
			name: "declared and read as off without elevation",
			result: host.CheckResult{Services: []host.Service{
				{Kind: host.ServiceRDP, Port: 3389},
			}},
			want:    "✗ NOT OK (rdp, port 3389)",
			notWant: "needs root/administrator",
		},
		{
			name: "declared and off under elevation",
			result: host.CheckResult{Elevated: true, Services: []host.Service{
				{Kind: host.ServiceRDP, Port: 3389},
			}},
			want:    "✗ NOT OK (rdp, port 3389)",
			notWant: "needs root/administrator",
		},
		{
			name: "unread under elevation is not claimed off",
			result: host.CheckResult{Elevated: true, Services: []host.Service{
				{Kind: host.ServiceRDP, Port: 3389, Unread: true},
			}},
			want:    "unread (rdp, port 3389): see the steps below",
			notWant: "NOT OK",
		},
		{
			name:    "declared with the port unread",
			result:  host.CheckResult{Services: []host.Service{{Kind: host.ServiceRDP}}},
			want:    "rdp, port unknown",
			notWant: "port 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmdpkg.PrintCheckTextForTest(&buf, tt.result)
			got := buf.String()
			if !strings.Contains(got, tt.want) {
				t.Errorf("output lacks %q:\n%s", tt.want, got)
			}
			if tt.notWant != "" && strings.Contains(got, tt.notWant) {
				t.Errorf("output contains %q:\n%s", tt.notWant, got)
			}
		})
	}
}

func TestPrintSetupSummary_Convergence(t *testing.T) {
	ok := host.SetupStep{Name: "firewall", Outcome: host.OutcomeOK, Success: true}
	changed := host.SetupStep{Name: "xrdp service", Outcome: host.OutcomeChanged, Success: true}
	manual := host.SetupStep{Name: "screen sharing", Outcome: host.OutcomeManual}
	failedX := host.SetupStep{Name: "xrdp service", Outcome: host.OutcomeFailed}
	failedIP := host.SetupStep{Name: "Tailscale IP", Outcome: host.OutcomeFailed}
	tests := []struct {
		name    string
		steps   []host.SetupStep
		want    []string
		notWant []string
	}{
		{name: "every step converged", steps: []host.SetupStep{ok, changed}, want: []string{"HOST READY"}, notWant: []string{"NOT READY"}},
		{name: "a manual step is not a failure", steps: []host.SetupStep{ok, manual}, want: []string{"HOST READY"}, notWant: []string{"NOT READY"}},
		{name: "no steps", want: []string{"HOST READY"}, notWant: []string{"NOT READY"}},
		{
			name:    "failed steps are named",
			steps:   []host.SetupStep{failedX, ok, failedIP},
			want:    []string{"HOST NOT READY", "xrdp service, Tailscale IP"},
			notWant: []string{"HOST READY"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmdpkg.PrintSetupSummaryForTest(&buf, "100.64.0.1", host.SetupResult{RDPPort: 3389, Steps: tt.steps})
			for _, want := range tt.want {
				if !strings.Contains(buf.String(), want) {
					t.Errorf("summary lacks %q:\n%s", want, buf.String())
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(buf.String(), notWant) {
					t.Errorf("summary contains %q:\n%s", notWant, buf.String())
				}
			}
		})
	}
}

// TestPrintSetupSummary_NoIP: without a Tailscale IP there is no client target
// and no descriptor to print, only the way to get one.
func TestPrintSetupSummary_NoIP(t *testing.T) {
	var buf bytes.Buffer
	cmdpkg.PrintSetupSummaryForTest(&buf, "", host.SetupResult{RDPPort: 3389})
	got := buf.String()
	for _, notWant := range []string{"TS_TARGET", "tsb://", "Tailscale IP:"} {
		if strings.Contains(got, notWant) {
			t.Errorf("summary contains %q:\n%s", notWant, got)
		}
	}
	if !strings.Contains(got, "target appears once Tailscale is up") {
		t.Errorf("summary does not say when the target appears:\n%s", got)
	}
}

// TestHostSetupCmd_NotConvergedBanner: the real command prints the not-ready
// banner and still returns the exit-2 error.
func TestHostSetupCmd_NotConvergedBanner(t *testing.T) {
	isolateHostEnv(t)
	cmdpkg.SetHostTailscaleIPForTest(t, "")
	c := cmdpkg.HostSetupCmdForTest(t, "linux", true, func(cfg host.Config, _ *slog.Logger) (host.SetupResult, error) {
		return host.SetupResult{RDPPort: cfg.Port, Steps: []host.SetupStep{
			{Name: "xrdp service", Outcome: host.OutcomeFailed},
			{Name: "Tailscale IP", Outcome: host.OutcomeFailed},
		}}, nil
	})
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(io.Discard)
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "2 of 2 steps failed") {
		t.Fatalf("err = %v, want the not-converged error", err)
	}
	for _, want := range []string{"HOST NOT READY", "xrdp service, Tailscale IP"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "TS_TARGET") {
		t.Errorf("stdout names a target without an address:\n%s", out.String())
	}
}
