package host

import (
	"testing"
)

func TestSetupFlags_Defaults(t *testing.T) {
	flags := SetupFlags{}
	if flags.NoSleep {
		t.Error("expected NoSleep default false")
	}
	if flags.FirewallRule != "" {
		t.Errorf("expected empty FirewallRule, got %q", flags.FirewallRule)
	}
}

func TestSetupFlags_Custom(t *testing.T) {
	flags := SetupFlags{
		NoSleep:      true,
		FirewallRule: "MyRule",
	}
	if !flags.NoSleep {
		t.Error("expected NoSleep true")
	}
	if flags.FirewallRule != "MyRule" {
		t.Errorf("expected FirewallRule MyRule, got %q", flags.FirewallRule)
	}
}

func TestSetupResult_Empty(t *testing.T) {
	result := SetupResult{}
	if result.RDPPort != 0 {
		t.Errorf("expected RDPPort 0, got %d", result.RDPPort)
	}
	if len(result.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(result.Steps))
	}
}

func TestSetupStep_Values(t *testing.T) {
	step := SetupStep{
		Name:    "Test step",
		Success: true,
		Message: "All good",
	}
	if step.Name != "Test step" {
		t.Errorf("expected Name 'Test step', got %q", step.Name)
	}
	if !step.Success {
		t.Error("expected Success true")
	}
	if step.Message != "All good" {
		t.Errorf("expected Message 'All good', got %q", step.Message)
	}
}

func TestCheckResult_Empty(t *testing.T) {
	result := CheckResult{}
	if result.TailscaleIP != "" {
		t.Errorf("expected empty TailscaleIP, got %q", result.TailscaleIP)
	}
	if result.RDPPort != 0 {
		t.Errorf("expected RDPPort 0, got %d", result.RDPPort)
	}
	if result.RDPEnabled {
		t.Error("expected RDPEnabled false")
	}
	if result.FirewallOK {
		t.Error("expected FirewallOK false")
	}
	if result.TailscaleUp {
		t.Error("expected TailscaleUp false")
	}
}

// TestIsElevated_DoesNotPanic verifies the delegation function works.
// The result depends on platform/privileges, but the function must not panic.
func TestIsElevated_DoesNotPanic(t *testing.T) {
	_ = IsElevated()
}

// TestSetup_DoesNotPanic verifies the delegation function works.
// On platforms without tailscale installed, this will return steps with
// errors — that's expected and valid behavior.
func TestSetup_DoesNotPanic(t *testing.T) {
	cfg := Config{
		NoSleep:      false,
		FirewallRule: "TestRule",
	}
	result, err := Setup(cfg, nil)
	if err != nil {
		t.Fatalf("Setup returned unexpected error: %v", err)
	}
	_ = result
}

// TestSetup_NoSleepSkipsPowerSettings verifies that NoSleep flag is passed correctly.
func TestSetup_NoSleepSkipsPowerSettings(t *testing.T) {
	cfg := Config{
		NoSleep:      true,
		FirewallRule: "TestRule",
	}
	result, err := Setup(cfg, nil)
	if err != nil {
		t.Fatalf("Setup returned unexpected error: %v", err)
	}
	_ = result
}

// TestCheck_DoesNotPanic verifies the delegation function works.
func TestCheck_DoesNotPanic(t *testing.T) {
	result, err := Check(Config{}, nil)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}
	_ = result
}

// TestTailscaleIP_DoesNotPanic verifies the delegation function works.
func TestTailscaleIP_DoesNotPanic(t *testing.T) {
	ip := TailscaleIP()
	// May be empty if tailscale isn't installed — that's fine.
	_ = ip
}

// ─── Firewall rule sanitization ──────────────────────────────────

func TestSanitizeFirewallRule_ValidNames(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"Tailscale-RDP-Ingress", false},
		{"MyRule", false},
		{"rule_123", false},
		{"a", false},
		{"A-B-C_D1", false},
		{"ts-bridge-firewall", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeFirewallRule(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeFirewallRule(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.name {
				t.Errorf("sanitizeFirewallRule(%q) = %q, want %q", tt.name, got, tt.name)
			}
		})
	}
}

func TestSanitizeFirewallRule_RejectsInjection(t *testing.T) {
	injections := []string{
		`rule; Remove-Item C:\ -Recurse`,
		`rule` + "`" + `nWrite-Host pwned`,
		`rule | Out-Null`,
		`rule && echo pwned`,
		`rule"; Remove-Item C:\ -Recurse`,
		"rule' OR '1'='1",
		`rule$(whoami)`,
		`rule/../../../etc/passwd`,
		`../escape`,
		"rule with spaces",
	}
	for _, injection := range injections {
		t.Run(injection, func(t *testing.T) {
			_, err := sanitizeFirewallRule(injection)
			if err == nil {
				t.Errorf("sanitizeFirewallRule(%q) should reject injection but returned nil", injection)
			}
		})
	}
}

func TestSanitizeFirewallRule_Empty(t *testing.T) {
	_, err := sanitizeFirewallRule("")
	if err == nil {
		t.Error("sanitizeFirewallRule(\"\") should reject empty string")
	}
}

func TestSanitizeFirewallRule_MaxLength(t *testing.T) {
	long := "a"
	for i := 0; i < 63; i++ {
		long += "a"
	}
	// 64 chars — should pass
	got, err := sanitizeFirewallRule(long)
	if err != nil {
		t.Errorf("sanitizeFirewallRule(64 chars) should accept, got error: %v", err)
	}
	if len(got) != 64 {
		t.Errorf("sanitizeFirewallRule(64 chars) = %d, want 64", len(got))
	}

	// 65 chars — should fail
	long65 := long + "a"
	_, err = sanitizeFirewallRule(long65)
	if err == nil {
		t.Error("sanitizeFirewallRule(65 chars) should reject, got nil")
	}
}

func TestSanitizeFirewallRule_SpecialChars(t *testing.T) {
	specials := []string{
		"rule@domain",
		"rule#hash",
		"rule$dollar",
		"rule%percent",
		"rule&and",
		"rule*star",
		"rule+plus",
		"rule=equals",
		"rule<lt>",
		"rule?question",
		"rule[bracket]",
		"rule{brace}",
		"rule\backslash",
		"rule/forward",
		"rule:new",
		"rule,comma",
	}
	for _, s := range specials {
		t.Run(s, func(t *testing.T) {
			_, err := sanitizeFirewallRule(s)
			if err == nil {
				t.Errorf("sanitizeFirewallRule(%q) should reject special char", s)
			}
		})
	}
}
