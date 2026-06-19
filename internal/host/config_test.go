// Package host provides platform-specific host setup and check operations.
package host

import (
	"testing"
)

// ─── Config merge chain tests ────────────────────────────────────

func TestMerge_Defaults(t *testing.T) {
	cfg := Merge(Flags{})
	if cfg.FirewallRule != defaultFirewallRuleName {
		t.Errorf("FirewallRule = %q, want %q", cfg.FirewallRule, defaultFirewallRuleName)
	}
	if cfg.Port != defaultRDPPort {
		t.Errorf("Port = %d, want %d", cfg.Port, defaultRDPPort)
	}
	if cfg.NoSleep != defaultNoSleep {
		t.Errorf("NoSleep = %v, want %v", cfg.NoSleep, defaultNoSleep)
	}
	if cfg.LogFormat != defaultLogFormat {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, defaultLogFormat)
	}
}

func TestMerge_FlagsOverrideDefaults(t *testing.T) {
	flags := Flags{
		FirewallRule: "CustomRule",
		Port:         3390,
		NoSleep:      true,
	}
	cfg := Merge(flags)
	if cfg.FirewallRule != "CustomRule" {
		t.Errorf("FirewallRule = %q, want %q", cfg.FirewallRule, "CustomRule")
	}
	if cfg.Port != 3390 {
		t.Errorf("Port = %d, want %d", cfg.Port, 3390)
	}
	if !cfg.NoSleep {
		t.Error("NoSleep = false, want true")
	}
}

func TestMerge_PortZeroIsIgnored(t *testing.T) {
	// Port 0 means "not set" — should fall back to default.
	flags := Flags{Port: 0}
	cfg := Merge(flags)
	if cfg.Port != defaultRDPPort {
		t.Errorf("Port = %d, want %d (port 0 should use default)", cfg.Port, defaultRDPPort)
	}
}

func TestMerge_EmptyFirewallRuleIsIgnored(t *testing.T) {
	// Empty string means "not set" — should fall back to default.
	flags := Flags{FirewallRule: ""}
	cfg := Merge(flags)
	if cfg.FirewallRule != defaultFirewallRuleName {
		t.Errorf("FirewallRule = %q, want %q (empty should use default)", cfg.FirewallRule, defaultFirewallRuleName)
	}
}

func TestMerge_VerboseFlag(t *testing.T) {
	flags := Flags{Verbose: true}
	cfg := Merge(flags)
	if !cfg.Verbose {
		t.Error("Verbose = false, want true")
	}
}

func TestMerge_LogFormatFlag(t *testing.T) {
	flags := Flags{LogFormat: "json"}
	cfg := Merge(flags)
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
}

// ─── LoadConfig tests ────────────────────────────────────────────

func TestLoadConfig_Defaults(t *testing.T) {
	// Ensure no TS_HOST_* env vars are set.
	t.Setenv("TS_HOST_FIREWALL_RULE", "")
	t.Setenv("TS_HOST_NO_SLEEP", "")
	t.Setenv("TS_HOST_RDP_PORT", "")
	t.Setenv("TS_HOST_VERBOSE", "")
	t.Setenv("TS_HOST_LOG_FORMAT", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.FirewallRule != defaultFirewallRuleName {
		t.Errorf("FirewallRule = %q, want %q", cfg.FirewallRule, defaultFirewallRuleName)
	}
	if cfg.Port != defaultRDPPort {
		t.Errorf("Port = %d, want %d", cfg.Port, defaultRDPPort)
	}
}

func TestLoadConfig_EnvVars(t *testing.T) {
	t.Setenv("TS_HOST_FIREWALL_RULE", "EnvRule")
	t.Setenv("TS_HOST_NO_SLEEP", "true")
	t.Setenv("TS_HOST_RDP_PORT", "3390")
	t.Setenv("TS_HOST_VERBOSE", "true")
	t.Setenv("TS_HOST_LOG_FORMAT", "json")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.FirewallRule != "EnvRule" {
		t.Errorf("FirewallRule = %q, want %q", cfg.FirewallRule, "EnvRule")
	}
	if !cfg.NoSleep {
		t.Error("NoSleep = false, want true")
	}
	if cfg.Port != 3390 {
		t.Errorf("Port = %d, want %d", cfg.Port, 3390)
	}
	if !cfg.Verbose {
		t.Error("Verbose = false, want true")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
}

func TestLoadConfig_InvalidPort(t *testing.T) {
	t.Setenv("TS_HOST_RDP_PORT", "99999")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	// Invalid port falls back to default (soft validation with warning).
	if cfg.Port != defaultRDPPort {
		t.Errorf("Port = %d, want %d (invalid port should fall back to default)", cfg.Port, defaultRDPPort)
	}
}

func TestLoadConfig_InvalidPortNonNumeric(t *testing.T) {
	t.Setenv("TS_HOST_RDP_PORT", "abc")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	// Non-numeric port falls back to default (soft validation with warning).
	if cfg.Port != defaultRDPPort {
		t.Errorf("Port = %d, want %d (non-numeric port should fall back to default)", cfg.Port, defaultRDPPort)
	}
}

func TestLoadConfig_ZeroPort(t *testing.T) {
	t.Setenv("TS_HOST_RDP_PORT", "0")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	// Port 0 falls back to default (soft validation with warning).
	if cfg.Port != defaultRDPPort {
		t.Errorf("Port = %d, want %d (port 0 should fall back to default)", cfg.Port, defaultRDPPort)
	}
}

func TestLoadConfig_InvalidFirewallRule(t *testing.T) {
	t.Setenv("TS_HOST_FIREWALL_RULE", "rule with spaces")
	_, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig() with invalid firewall rule should error")
	}
}



func TestLoadConfig_Port1(t *testing.T) {
	t.Setenv("TS_HOST_RDP_PORT", "1")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Port != 1 {
		t.Errorf("Port = %d, want 1", cfg.Port)
	}
}

func TestLoadConfig_Port65535(t *testing.T) {
	t.Setenv("TS_HOST_RDP_PORT", "65535")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Port != 65535 {
		t.Errorf("Port = %d, want 65535", cfg.Port)
	}
}

func TestLoadConfig_TruthyValues(t *testing.T) {
	for _, val := range []string{"1", "true", "yes", "on", "TRUE", "True"} {
		t.Run(val, func(t *testing.T) {
			t.Setenv("TS_HOST_NO_SLEEP", val)
			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if !cfg.NoSleep {
				t.Errorf("NoSleep with %q = false, want true", val)
			}
		})
	}
}

func TestLoadConfig_FalsyValues(t *testing.T) {
	for _, val := range []string{"0", "false", "no", "off", "random"} {
		t.Run(val, func(t *testing.T) {
			t.Setenv("TS_HOST_NO_SLEEP", val)
			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if cfg.NoSleep {
				t.Errorf("NoSleep with %q = true, want false", val)
			}
		})
	}
}

func TestLoadConfig_InvalidLogFormat(t *testing.T) {
	t.Setenv("TS_HOST_LOG_FORMAT", "xml")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	// Invalid log format should still return a config but with the invalid value
	// (validation is a soft warning, not a hard error for log format).
	_ = cfg
}

// ─── Default accessor tests ──────────────────────────────────────

func TestDefaultRDPPort(t *testing.T) {
	if DefaultRDPPort() != 3389 {
		t.Errorf("DefaultRDPPort() = %d, want 3389", DefaultRDPPort())
	}
}

func TestDefaultFirewallRule(t *testing.T) {
	if DefaultFirewallRule() != "Tailscale-RDP-Ingress" {
		t.Errorf("DefaultFirewallRule() = %q, want %q", DefaultFirewallRule(), "Tailscale-RDP-Ingress")
	}
}

func TestDefaultNoSleep(t *testing.T) {
	if DefaultNoSleep() != false {
		t.Errorf("DefaultNoSleep() = %v, want false", DefaultNoSleep())
	}
}

func TestDefaultVerbose(t *testing.T) {
	if DefaultVerbose() != false {
		t.Errorf("DefaultVerbose() = %v, want false", DefaultVerbose())
	}
}

func TestDefaultLogFormat(t *testing.T) {
	if DefaultLogFormat() != "text" {
		t.Errorf("DefaultLogFormat() = %q, want %q", DefaultLogFormat(), "text")
	}
}

// ─── parseBoolEnv tests ──────────────────────────────────────────

func TestParseBoolEnv(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"ON", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"off", false},
		{"random", false},
		{"", false},
		{"  true  ", true}, // trimmed
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseBoolEnv(tt.input)
			if got != tt.want {
				t.Errorf("parseBoolEnv(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
