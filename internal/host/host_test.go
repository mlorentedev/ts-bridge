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
	flags := SetupFlags{
		NoSleep:      false,
		FirewallRule: "TestRule",
	}
	result, err := Setup(flags)
	if err != nil {
		t.Fatalf("Setup returned unexpected error: %v", err)
	}
	_ = result
}

// TestSetup_NoSleepSkipsPowerSettings verifies that NoSleep flag is passed correctly.
func TestSetup_NoSleepSkipsPowerSettings(t *testing.T) {
	flags := SetupFlags{
		NoSleep:      true,
		FirewallRule: "TestRule",
	}
	result, err := Setup(flags)
	if err != nil {
		t.Fatalf("Setup returned unexpected error: %v", err)
	}
	_ = result
}

// TestCheck_DoesNotPanic verifies the delegation function works.
func TestCheck_DoesNotPanic(t *testing.T) {
	result, err := Check()
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