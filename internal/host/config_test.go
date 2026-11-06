// Package host provides platform-specific host setup and check operations.
package host

import (
	"slices"
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
		NoSleepSet:   true,
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
	flags := Flags{Verbose: true, VerboseSet: true}
	cfg := Merge(flags)
	if !cfg.Verbose {
		t.Error("Verbose = false, want true")
	}
}

// TestMerge_BoolFlagPrecedence guards the flags > env > defaults chain for the
// bool flags (no-sleep, verbose): an unset flag must NOT clobber an env value,
// and an explicitly-passed flag must win over env. Regression test for the bug
// where applyFlags unconditionally assigned cfg.NoSleep = flags.NoSleep.
func TestMerge_BoolFlagPrecedence(t *testing.T) {
	t.Run("unset flag does not override env", func(t *testing.T) {
		t.Setenv("TS_HOST_NO_SLEEP", "true")
		t.Setenv("TS_HOST_VERBOSE", "true")
		// Flag not passed: NoSleepSet/VerboseSet are false.
		cfg := Merge(Flags{})
		if !cfg.NoSleep {
			t.Error("NoSleep = false, want true (env should win when flag unset)")
		}
		if !cfg.Verbose {
			t.Error("Verbose = false, want true (env should win when flag unset)")
		}
	})

	t.Run("explicit flag overrides env", func(t *testing.T) {
		t.Setenv("TS_HOST_NO_SLEEP", "true")
		t.Setenv("TS_HOST_VERBOSE", "true")
		// Flag explicitly passed as false (the ambiguous case).
		cfg := Merge(Flags{NoSleep: false, NoSleepSet: true, Verbose: false, VerboseSet: true})
		if cfg.NoSleep {
			t.Error("NoSleep = true, want false (explicit flag should win over env)")
		}
		if cfg.Verbose {
			t.Error("Verbose = true, want false (explicit flag should win over env)")
		}
	})
}

func TestMerge_LogFormatFlag(t *testing.T) {
	flags := Flags{LogFormat: "json"}
	cfg := Merge(flags)
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
}

// ─── Resolve tests (HOST-005 A2) ─────────────────────────────────

// clearHostEnv unsets every TS_HOST_* variable Resolve reads.
func clearHostEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"TS_HOST_FIREWALL_RULE", "TS_HOST_NO_SLEEP", "TS_HOST_RDP_PORT", "TS_HOST_VERBOSE", "TS_HOST_LOG_FORMAT"} {
		t.Setenv(k, "")
	}
}

func ptr[T any](v T) *T { return &v }

func TestResolve_PerOSDefaults(t *testing.T) {
	clearHostEnv(t)
	rdp := []DeclaredService{{Kind: ServiceRDP, Port: 3389}}
	tests := []struct {
		name string
		goos string
		file File
		want []DeclaredService
	}{
		{"windows, no file", goosWindows, File{}, rdp},
		{"linux, no file", goosLinux, File{}, rdp},
		{"darwin, no file", goosDarwin, File{}, []DeclaredService{{Kind: ServiceSSH, Port: 22}}},
		{"host without services keeps the OS default", goosDarwin, File{Version: 1, Host: FileHost{ControlURL: ptr("https://hs.example")}}, []DeclaredService{{Kind: ServiceSSH, Port: 22}}},
		{"rdp without port takes the kind default", goosLinux, File{Host: FileHost{Services: []FileService{{Kind: ServiceRDP}}}}, rdp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Resolve(tt.goos, tt.file, Flags{})
			if !slices.Equal(cfg.Services, tt.want) {
				t.Errorf("Services = %v, want %v", cfg.Services, tt.want)
			}
			if cfg.Port != defaultRDPPort {
				t.Errorf("Port = %d, want %d on every OS (legacy field)", cfg.Port, defaultRDPPort)
			}
		})
	}
}

func TestResolve_Precedence(t *testing.T) {
	file := File{Host: FileHost{
		Services:   []FileService{{Kind: ServiceRDP, Port: ptr(3391)}},
		Firewall:   &FileFirewall{Rule: ptr("FileRule")},
		Power:      &FilePower{PreventSleep: ptr(false)},
		ControlURL: ptr("https://hs.example"),
	}}
	tests := []struct {
		name      string
		file      File
		env       map[string]string
		flags     Flags
		wantPort  int
		wantRule  string
		wantSleep bool // NoSleep
	}{
		{"defaults", File{}, nil, Flags{}, 3389, defaultFirewallRuleName, false},
		{"file beats defaults", file, nil, Flags{}, 3391, "FileRule", true},
		{"env beats file", file,
			map[string]string{"TS_HOST_RDP_PORT": "3392", "TS_HOST_FIREWALL_RULE": "EnvRule", "TS_HOST_NO_SLEEP": "false"},
			Flags{}, 3392, "EnvRule", false},
		{"flags beat env", file,
			map[string]string{"TS_HOST_RDP_PORT": "3392", "TS_HOST_FIREWALL_RULE": "EnvRule", "TS_HOST_NO_SLEEP": "false"},
			Flags{Port: 3393, FirewallRule: "FlagRule", NoSleep: true, NoSleepSet: true}, 3393, "FlagRule", true},
		{"prevent_sleep true keeps sleep disabled", File{Host: FileHost{Power: &FilePower{PreventSleep: ptr(true)}}}, nil, Flags{}, 3389, defaultFirewallRuleName, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearHostEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg := Resolve(goosWindows, tt.file, tt.flags)
			if cfg.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", cfg.Port, tt.wantPort)
			}
			if want := []DeclaredService{{Kind: ServiceRDP, Port: tt.wantPort}}; !slices.Equal(cfg.Services, want) {
				t.Errorf("Services = %v, want %v (rdp follows Port)", cfg.Services, want)
			}
			if cfg.FirewallRule != tt.wantRule {
				t.Errorf("FirewallRule = %q, want %q", cfg.FirewallRule, tt.wantRule)
			}
			if cfg.NoSleep != tt.wantSleep {
				t.Errorf("NoSleep = %v, want %v", cfg.NoSleep, tt.wantSleep)
			}
		})
	}

	t.Run("control_url comes from the file only", func(t *testing.T) {
		clearHostEnv(t)
		t.Setenv("TS_CONTROL_URL", "https://env.invalid")
		if got := Resolve(goosLinux, File{}, Flags{}).ControlURL; got != "" {
			t.Errorf("ControlURL without file = %q, want empty", got)
		}
		if got := Resolve(goosLinux, File{Host: FileHost{ControlURL: ptr("https://hs.example")}}, Flags{}).ControlURL; got != "https://hs.example" {
			t.Errorf("ControlURL = %q, want the file's", got)
		}
	})
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
	// Invalid log format is a soft warning (not a hard error) and falls back to
	// the default, as documented on LoadConfig.
	if cfg.LogFormat != defaultLogFormat {
		t.Errorf("LogFormat = %q, want fallback %q", cfg.LogFormat, defaultLogFormat)
	}
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

// PortDeclared says whether any layer named the RDP port, so a step can tell a
// declaration from the 3389 default (#557).
func TestResolvePortDeclared(t *testing.T) {
	port := 3390
	tests := []struct {
		name  string
		env   string
		file  File
		flags Flags
		want  bool
	}{
		{name: "default"},
		{name: "rdp service without a port", file: File{Host: FileHost{Services: []FileService{{Kind: ServiceRDP}}}}},
		{name: "file port", file: File{Host: FileHost{Services: []FileService{{Kind: ServiceRDP, Port: &port}}}}, want: true},
		{name: "env", env: "3390", want: true},
		{name: "invalid env", env: "abc"},
		{name: "flag", flags: Flags{Port: 3390}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TS_HOST_RDP_PORT", tt.env)
			if got := Resolve("windows", tt.file, tt.flags).PortDeclared; got != tt.want {
				t.Errorf("PortDeclared = %v, want %v", got, tt.want)
			}
		})
	}
}
