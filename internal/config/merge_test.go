package config

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// --- Merge precedence tests ---

func TestMergeFlagOverridesEnv(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-env")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	yaml := PartialConfig{Target: "100.64.0.2:443"}
	flags := FlagSet{Target: "100.64.0.3:8080"}

	cfg, err := Merge(yaml, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Target != "100.64.0.3:8080" {
		t.Errorf("expected flag target 100.64.0.3:8080, got %s", cfg.Target)
	}
}

func TestMergeEnvOverridesYAML(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-env")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	yaml := PartialConfig{Target: "100.64.0.2:443"}

	cfg, err := Merge(yaml, FlagSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Target != "100.64.0.1:3389" {
		t.Errorf("expected env target 100.64.0.1:3389, got %s", cfg.Target)
	}
}

func TestMergeYAMLOverridesDefault(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-env")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	yaml := PartialConfig{Timeout: mustParseDuration("2m")}

	cfg, err := Merge(yaml, FlagSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ConnectTimeout != 2*time.Minute {
		t.Errorf("expected timeout 2m, got %v", cfg.ConnectTimeout)
	}
}

func TestMergeFullPrecedence(t *testing.T) {
	os.Setenv("TS_TIMEOUT", "1m")
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-env")
	defer os.Unsetenv("TS_TIMEOUT")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	yaml := PartialConfig{Timeout: mustParseDuration("3m")}
	// 45s, not 30s: the flag value must differ from defaultTimeout (30s), or a
	// passing assertion would also be consistent with the flag layer never
	// having been applied at all.
	flags := FlagSet{Timeout: mustParseDuration("45s")}

	cfg, err := Merge(yaml, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Flag (45s) > Env (1m) > YAML (3m) > Default (30s)
	if cfg.ConnectTimeout != 45*time.Second {
		t.Errorf("expected flag timeout 45s, got %v", cfg.ConnectTimeout)
	}
}

func TestMergeMissingYAMLNotError(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-env")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	// No YAML, no flags — should use env + defaults
	cfg, err := Merge(PartialConfig{}, FlagSet{})
	if err != nil {
		t.Fatalf("unexpected error for empty YAML: %v", err)
	}
	if cfg.Target != "100.64.0.1:3389" {
		t.Errorf("expected env target, got %s", cfg.Target)
	}
}

func TestMergeAuthKeyNotInYAML(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-env")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	// YAML must NOT carry auth key — if it does, Merge should reject it
	yaml := PartialConfig{AuthKey: "tskey-auth-yaml"}
	_, err := Merge(yaml, FlagSet{})
	if err == nil {
		t.Fatal("expected error when YAML contains auth key")
	}
	if !strings.Contains(err.Error(), "auth") {
		t.Errorf("error should mention auth, got: %v", err)
	}
}

func TestMergeUnknownYAMLFieldsWarn(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-env")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	// Unknown field in YAML should produce a warning (not an error)
	// We test via LoadYAMLConfig with a file containing unknown fields
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
version: 1
target: "100.64.0.1:3389"
unknown_field: "should_warn"
`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadYAMLConfig(yamlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = cfg // unknown fields should not cause error, just warning
}

func TestMergeConfigFromFile(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-env")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
version: 1
target: "100.64.0.2:443"
timeout: 2m
hostname: "yaml-host"
`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadYAMLConfig(yamlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Target != "100.64.0.2:443" {
		t.Errorf("expected target from YAML, got %s", cfg.Target)
	}
	if cfg.Timeout != 2*time.Minute {
		t.Errorf("expected timeout 2m from YAML, got %v", cfg.Timeout)
	}
}

func TestMergeMissingYAMLFileIsError(t *testing.T) {
	// Missing explicit YAML file should error
	_, err := LoadYAMLConfig("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("missing explicit YAML file should error")
	}
}

func TestMergeYAMLWorldReadableWarn(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping: running as root, permissions not enforced")
	}

	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	// Write with 0600 first, then chmod to 0644 for world-readable test.
	if err := os.WriteFile(yamlPath, []byte(`
version: 1
target: "100.64.0.1:3389"
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(yamlPath, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadYAMLConfig(yamlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = cfg // should produce warning but not error
}

func TestMergeYAMLRejectsAuthKey(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
version: 1
target: "100.64.0.1:3389"
auth_key: "tskey-auth-secret"
`), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadYAMLConfig(yamlPath)
	if err == nil {
		t.Fatal("expected error when YAML contains auth_key")
	}
	if !strings.Contains(err.Error(), "auth") {
		t.Errorf("error should mention auth, got: %v", err)
	}
}

func TestMergeYAMLVersionField(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
version: 1
target: "100.64.0.1:3389"
`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadYAMLConfig(yamlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = cfg // version: 1 should be accepted
}

func TestMergeYAMLMissingVersionUsesDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
target: "100.64.0.1:3389"
`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadYAMLConfig(yamlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = cfg // missing version should use defaults
	_ = cfg
}

// --- FlagSet tests ---

func TestFlagSetTarget(t *testing.T) {
	flags := FlagSet{Target: "100.64.0.1:3389"}
	if flags.Target != "100.64.0.1:3389" {
		t.Errorf("expected target 100.64.0.1:3389, got %s", flags.Target)
	}
}

func TestFlagSetEmptyByDefault(t *testing.T) {
	flags := FlagSet{}
	if flags.Target != "" {
		t.Errorf("expected empty target, got %s", flags.Target)
	}
}

// --- BUG-009: default hostname in manual-mode ---

func TestMergeDefaultHostnameInManualMode(t *testing.T) {
	os.Unsetenv("TS_HOSTNAME")
	os.Unsetenv("TS_TARGET")
	os.Unsetenv("TS_AUTHKEY")

	flags := FlagSet{
		Target:     "100.64.0.1:3389",
		AuthKey:    "tskey-test",
		ManualMode: true,
	}

	cfg, err := Merge(PartialConfig{}, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Hostname == "" {
		t.Fatal("Hostname should not be empty in manual-mode; expected default 'ts-bridge'")
	}
	if cfg.Hostname != "ts-bridge" {
		t.Errorf("expected default hostname 'ts-bridge', got %q", cfg.Hostname)
	}
}

func TestMergeExplicitHostnameOverridesDefault(t *testing.T) {
	os.Unsetenv("TS_HOSTNAME")
	os.Unsetenv("TS_TARGET")
	os.Unsetenv("TS_AUTHKEY")

	flags := FlagSet{
		Target:     "100.64.0.1:3389",
		AuthKey:    "tskey-test",
		Hostname:   "my-server",
		ManualMode: true,
	}

	cfg, err := Merge(PartialConfig{}, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Hostname != "my-server" {
		t.Errorf("expected explicit hostname 'my-server', got %q", cfg.Hostname)
	}
}

func TestMergeAutoModeDerivesHostname(t *testing.T) {
	os.Unsetenv("TS_HOSTNAME")
	os.Unsetenv("TS_TARGET")
	os.Unsetenv("TS_AUTHKEY")

	flags := FlagSet{
		Target:   "100.64.0.1:3389",
		AuthKey:  "tskey-test",
		Instance: "test-instance",
	}

	cfg, err := Merge(PartialConfig{}, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Hostname == "" {
		t.Fatal("Hostname should be derived in auto-mode")
	}
	if cfg.Hostname == "ts-bridge" {
		t.Error("auto-mode should derive a specific hostname, not use the default")
	}
}

// --- BUG-021: auto-mode always derives LocalAddr ---

func TestMergeAutoModeDerivesLocalAddr(t *testing.T) {
	// When no LocalAddr flag/env is set, auto-mode must derive one.
	os.Unsetenv("TS_LOCAL_ADDR")
	os.Unsetenv("TS_HOSTNAME")
	os.Unsetenv("TS_TARGET")
	os.Unsetenv("TS_AUTHKEY")
	os.Unsetenv("TS_PORT_RANGE")

	flags := FlagSet{
		Target:  "100.64.0.1:3389",
		AuthKey: "tskey-test",
	}

	cfg, err := Merge(PartialConfig{}, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LocalAddr == "" {
		t.Fatal("LocalAddr should be derived in auto-mode (BUG-021)")
	}
	if !strings.HasPrefix(cfg.LocalAddr, "127.0.0.1:") {
		t.Errorf("LocalAddr should be 127.0.0.1:PORT, got %q", cfg.LocalAddr)
	}
	if !cfg.AutoInstance {
		t.Error("AutoInstance should be true in default auto-mode")
	}
}

func TestMergeAutoModeLocalAddrFromPortRangeFlag(t *testing.T) {
	// When --port-range is set, derive from that range.
	os.Unsetenv("TS_LOCAL_ADDR")
	os.Unsetenv("TS_HOSTNAME")
	os.Unsetenv("TS_TARGET")
	os.Unsetenv("TS_AUTHKEY")
	os.Unsetenv("TS_PORT_RANGE")

	flags := FlagSet{
		Target:    "100.64.0.1:3389",
		AuthKey:   "tskey-test",
		PortRange: "40000-41000",
	}

	cfg, err := Merge(PartialConfig{}, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LocalAddr == "" {
		t.Fatal("LocalAddr should be derived from --port-range")
	}
	// Verify port is in the requested range.
	_, portStr, err := net.SplitHostPort(cfg.LocalAddr)
	if err != nil {
		t.Fatalf("invalid LocalAddr: %v", err)
	}
	port, _ := strconv.Atoi(portStr)
	if port < 40000 || port > 41000 {
		t.Errorf("LocalAddr port %d not in range 40000-41000", port)
	}
}

func TestMergeAutoModeLocalAddrFromEnv(t *testing.T) {
	// When TS_LOCAL_ADDR is set, use it (no derivation).
	os.Setenv("TS_LOCAL_ADDR", "127.0.0.1:9999")
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-test")
	defer os.Unsetenv("TS_LOCAL_ADDR")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	cfg, err := Merge(PartialConfig{}, FlagSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LocalAddr != "127.0.0.1:9999" {
		t.Errorf("LocalAddr should be from env, got %q", cfg.LocalAddr)
	}
}

func TestMergeManualModeNoLocalAddrDerived(t *testing.T) {
	// Manual mode: LocalAddr stays empty (user must set it).
	os.Unsetenv("TS_LOCAL_ADDR")
	os.Unsetenv("TS_HOSTNAME")
	os.Unsetenv("TS_TARGET")
	os.Unsetenv("TS_AUTHKEY")
	os.Unsetenv("TS_PORT_RANGE")

	flags := FlagSet{
		Target:     "100.64.0.1:3389",
		AuthKey:    "tskey-test",
		ManualMode: true,
	}

	cfg, err := Merge(PartialConfig{}, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoInstance {
		t.Error("AutoInstance should be false in manual mode")
	}
	if cfg.LocalAddr != "" {
		t.Errorf("LocalAddr should be empty in manual mode, got %q", cfg.LocalAddr)
	}
}

// --- BUG-005: validation order tests ---

func TestMergeValidationOrder_TargetFormatBeforeAuthKey(t *testing.T) {
	// When both target is invalid AND auth key is missing,
	// target format error should surface first (not auth key error).
	os.Unsetenv("TS_AUTHKEY")
	os.Unsetenv("TS_TARGET")

	flags := FlagSet{Target: "invalid-no-port"}

	_, err := Merge(PartialConfig{}, flags)
	if err == nil {
		t.Fatal("expected error for invalid target")
	}
	if !strings.Contains(err.Error(), "target") {
		t.Errorf("expected error to mention 'target', got: %v", err)
	}
	if strings.Contains(err.Error(), "auth") {
		t.Errorf("auth key error should not mask target error, got: %v", err)
	}
}

func TestMergeValidationOrder_InvalidTargetWithMissingAuthKey(t *testing.T) {
	os.Unsetenv("TS_AUTHKEY")
	os.Unsetenv("TS_TARGET")

	flags := FlagSet{Target: "no-port-here"}

	_, err := Merge(PartialConfig{}, flags)
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	// First error should be about target format, not auth key
	if !strings.Contains(errStr, "target invalid") {
		t.Errorf("first error should be 'target invalid', got: %v", err)
	}
}

func TestMergeValidationOrder_InvalidTargetWithMissingAuthKeyEnv(t *testing.T) {
	os.Unsetenv("TS_AUTHKEY")
	os.Unsetenv("TS_TARGET")

	flags := FlagSet{Target: "100.64.0.1"} // missing port

	_, err := Merge(PartialConfig{}, flags)
	if err == nil {
		t.Fatal("expected error for target missing port")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "target") {
		t.Errorf("expected target error, got: %v", err)
	}
	if strings.Contains(errStr, "auth") {
		t.Errorf("auth key error should not appear when target is invalid, got: %v", err)
	}
}

func TestMergeValidationOrder_ValidTargetMissingAuthKey(t *testing.T) {
	os.Unsetenv("TS_AUTHKEY")
	os.Unsetenv("TS_TARGET")

	flags := FlagSet{Target: "100.64.0.1:3389"}

	_, err := Merge(PartialConfig{}, flags)
	if err == nil {
		t.Fatal("expected error for missing auth key")
	}
	errStr := err.Error()
	// Target is valid, so auth key error should appear
	if !strings.Contains(errStr, "auth") {
		t.Errorf("expected auth key error when target is valid, got: %v", err)
	}
}

func TestMergeValidationOrder_MissingTargetMissingAuthKey(t *testing.T) {
	os.Unsetenv("TS_AUTHKEY")
	os.Unsetenv("TS_TARGET")

	_, err := Merge(PartialConfig{}, FlagSet{})
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
	errStr := err.Error()
	// Target required should fire before auth key required
	if !strings.Contains(errStr, "target") {
		t.Errorf("expected target error first, got: %v", err)
	}
}

func TestMergeValidationOrder_InvalidAuthKeyFormat(t *testing.T) {
	os.Unsetenv("TS_AUTHKEY")
	os.Unsetenv("TS_TARGET")

	flags := FlagSet{
		Target:  "100.64.0.1:3389",
		AuthKey: "invalid-key-format",
	}

	_, err := Merge(PartialConfig{}, flags)
	if err == nil {
		t.Fatal("expected error for invalid auth key format")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "auth key invalid format") {
		t.Errorf("expected auth key format error, got: %v", err)
	}
}

// --- BUG-006: dial-retries validation tests ---

func TestMergeDialRetriesValidation(t *testing.T) {
	tests := []struct {
		name           string
		envDialRetries string
		// nil means the user did not pass --dial-retries at all; ptr(0) means
		// they passed it explicitly as 0. Before #282 these were the same value
		// and the unset case silently clobbered env and YAML.
		flagDialRetries *int
		wantErr         bool
		wantRetries     int
		errContains     string
	}{
		{
			name:            "negative flag rejected",
			flagDialRetries: ptr(-1),
			wantErr:         true,
			errContains:     "dial retries must be non-negative",
		},
		{
			name:            "zero flag accepted",
			flagDialRetries: ptr(0),
			wantErr:         false,
			wantRetries:     0,
		},
		{
			name:            "positive flag accepted",
			flagDialRetries: ptr(5),
			wantErr:         false,
			wantRetries:     5,
		},
		{
			name:           "negative env rejected",
			envDialRetries: "-1",
			wantErr:        true,
			errContains:    "dial retries must be non-negative",
		},
		{
			name:            "negative env with flag 0 rejected",
			envDialRetries:  "-5",
			flagDialRetries: ptr(0),
			wantErr:         true,
			errContains:     "dial retries must be non-negative",
		},
		{
			name:            "flag overrides env (flag 3, env 10)",
			envDialRetries:  "10",
			flagDialRetries: ptr(3),
			wantErr:         false,
			wantRetries:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TS_TARGET", "100.64.0.1:3389")
			os.Setenv("TS_AUTHKEY", "tskey-auth-env")
			defer os.Unsetenv("TS_TARGET")
			defer os.Unsetenv("TS_AUTHKEY")

			if tt.envDialRetries != "" {
				os.Setenv("TS_DIAL_RETRIES", tt.envDialRetries)
				defer os.Unsetenv("TS_DIAL_RETRIES")
			}

			cfg, err := Merge(PartialConfig{}, FlagSet{DialRetries: tt.flagDialRetries})
			if (err != nil) != tt.wantErr {
				t.Errorf("Merge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error should contain %q, got: %v", tt.errContains, err)
				}
				return
			}
			if err == nil && cfg.DialRetries != tt.wantRetries {
				t.Errorf("expected DialRetries %d, got %d", tt.wantRetries, cfg.DialRetries)
			}
		})
	}
}

// --- BUG-020: AutoInstance resolution in Merge() ---

func TestMergeManualModeFlagDisablesAutoInstance(t *testing.T) {
	os.Unsetenv("TS_AUTO_INSTANCE")
	os.Unsetenv("TS_MANUAL_MODE")
	os.Unsetenv("TS_TARGET")
	os.Unsetenv("TS_AUTHKEY")

	flags := FlagSet{
		Target:     "100.64.0.1:3389",
		AuthKey:    "tskey-test",
		ManualMode: true,
	}

	cfg, err := Merge(PartialConfig{}, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoInstance {
		t.Error("AutoInstance should be false when --manual-mode flag is set")
	}
	// In manual mode, LocalAddr/Hostname should NOT be auto-derived.
	if cfg.LocalAddr != "" {
		t.Errorf("LocalAddr should be empty in manual mode without explicit flag, got %q", cfg.LocalAddr)
	}
	if cfg.Hostname != "ts-bridge" {
		t.Errorf("Hostname should be default 'ts-bridge' in manual mode, got %q", cfg.Hostname)
	}
}

func TestMergeEnvAutoInstanceFalse(t *testing.T) {
	os.Setenv("TS_AUTO_INSTANCE", "false")
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-test")
	defer os.Unsetenv("TS_AUTO_INSTANCE")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	cfg, err := Merge(PartialConfig{}, FlagSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoInstance {
		t.Error("AutoInstance should be false when TS_AUTO_INSTANCE=false")
	}
}

func TestMergeEnvManualModeTrue(t *testing.T) {
	os.Setenv("TS_MANUAL_MODE", "true")
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-test")
	defer os.Unsetenv("TS_MANUAL_MODE")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	cfg, err := Merge(PartialConfig{}, FlagSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoInstance {
		t.Error("AutoInstance should be false when TS_MANUAL_MODE=true")
	}
}

func TestMergeFlagManualModeOverridesEnvAutoInstance(t *testing.T) {
	// Env says auto, flag says manual → flag wins.
	os.Setenv("TS_AUTO_INSTANCE", "true")
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-test")
	defer os.Unsetenv("TS_AUTO_INSTANCE")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	flags := FlagSet{
		Target:     "100.64.0.1:3389",
		AuthKey:    "tskey-test",
		ManualMode: true,
	}

	cfg, err := Merge(PartialConfig{}, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoInstance {
		t.Error("--manual-mode flag should override TS_AUTO_INSTANCE=true")
	}
}

func TestMergeYAMLAutoInstanceExplicit(t *testing.T) {
	os.Unsetenv("TS_AUTO_INSTANCE")
	os.Unsetenv("TS_MANUAL_MODE")
	os.Unsetenv("TS_TARGET")
	os.Unsetenv("TS_AUTHKEY")

	yamlCfg := PartialConfig{
		Target:       "100.64.0.1:3389",
		AutoInstance: boolPtr(false),
	}

	cfg, err := Merge(yamlCfg, FlagSet{AuthKey: "tskey-test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoInstance {
		t.Error("AutoInstance should be false when YAML sets auto_instance: false")
	}
}

func TestMergeAutoInstanceEnvOverridesYAML(t *testing.T) {
	// Env should override YAML for AutoInstance.
	os.Setenv("TS_AUTO_INSTANCE", "true")
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-test")
	defer os.Unsetenv("TS_AUTO_INSTANCE")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	yamlCfg := PartialConfig{
		Target:       "100.64.0.1:3389",
		AutoInstance: boolPtr(false),
	}

	cfg, err := Merge(yamlCfg, FlagSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.AutoInstance {
		t.Error("TS_AUTO_INSTANCE=true env should override YAML auto_instance: false")
	}
}

func TestMergeManualModeNoAutoDerivedValues(t *testing.T) {
	// In manual mode, LocalAddr/Hostname/StateDir should NOT be auto-derived.
	os.Unsetenv("TS_AUTO_INSTANCE")
	os.Unsetenv("TS_MANUAL_MODE")
	os.Unsetenv("TS_LOCAL_ADDR")
	os.Unsetenv("TS_HOSTNAME")
	os.Unsetenv("TS_STATE_DIR")
	os.Unsetenv("TS_TARGET")
	os.Unsetenv("TS_AUTHKEY")

	flags := FlagSet{
		Target:     "100.64.0.1:3389",
		AuthKey:    "tskey-test",
		ManualMode: true,
	}

	cfg, err := Merge(PartialConfig{}, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoInstance {
		t.Error("AutoInstance should be false in manual mode")
	}
	if cfg.EphemeralState {
		t.Error("EphemeralState should be false in manual mode (no auto-derived StateDir)")
	}
	// LocalAddr should be empty (no auto-derivation, no env, no flag).
	// The default LocalAddr is applied by LoadConfig(), not Merge().
	if cfg.LocalAddr != "" {
		t.Errorf("LocalAddr should be empty in manual mode without explicit value, got %q", cfg.LocalAddr)
	}
}

func boolPtr(b bool) *bool {
	return &b
}

// --- Helper ---

func mustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

// --- STATE-001: fixed per-user state directory (#207) ---

// stateEnvKeys are the env vars that influence state-dir / auto-mode resolution.
var stateEnvKeys = []string{
	"TS_STATE_DIR", "TS_AUTO_INSTANCE", "TS_MANUAL_MODE",
	"TS_LOCAL_ADDR", "TS_HOSTNAME", "TS_INSTANCE_NAME", "TS_PORT_RANGE",
	"TS_TARGET", "TS_AUTHKEY", "TS_CONTROL_URL",
}

// restoreStateEnv snapshots and clears every state-influencing env var, then
// restores the originals via t.Cleanup so a case never leaks into later tests.
func restoreStateEnv(t *testing.T) {
	t.Helper()
	for _, k := range stateEnvKeys {
		if prev, ok := os.LookupEnv(k); ok {
			k, prev := k, prev
			t.Cleanup(func() { os.Setenv(k, prev) })
		} else {
			k := k
			t.Cleanup(func() { os.Unsetenv(k) })
		}
		os.Unsetenv(k)
	}
}

// baseFlags returns a minimal valid FlagSet (target + auth key) merged with the
// case-specific overrides.
func baseFlags(extra FlagSet) FlagSet {
	extra.Target = "100.64.0.1:3389"
	extra.AuthKey = "tskey-test"
	return extra
}

// State-dir resolution contract: the default is a fixed per-user absolute path
// (never ./ts-state); auto-mode-with-instance is ephemeral under temp; explicit
// relative overrides are preserved verbatim.
func TestStateDirResolution(t *testing.T) {
	cases := []struct {
		name          string
		yaml          PartialConfig
		flags         FlagSet
		want          func(cfg Config) string
		wantEphemeral bool
	}{
		{
			name:  "manual mode default is fixed per-user",
			flags: FlagSet{ManualMode: true},
			want:  func(Config) string { return StateDirForPlatform() },
		},
		{
			name:  "auto mode without instance is fixed per-user",
			flags: FlagSet{},
			want:  func(Config) string { return StateDirForPlatform() },
		},
		{
			name:          "auto mode with instance is ephemeral under temp",
			flags:         FlagSet{Instance: "office"},
			want:          func(cfg Config) string { return EphemeralStateDir(cfg.Hostname) },
			wantEphemeral: true,
		},
		{
			name:  "explicit relative flag override preserved (legacy opt-in)",
			flags: FlagSet{StateDir: "./ts-state", ManualMode: true},
			want:  func(Config) string { return "./ts-state" },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreStateEnv(t)
			cfg, err := Merge(tc.yaml, baseFlags(tc.flags))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.StateDir == "./ts-state" && tc.flags.StateDir == "" {
				t.Fatal("resolved state dir is still the CWD-relative ./ts-state (#207 not fixed)")
			}
			if want := tc.want(cfg); cfg.StateDir != want {
				t.Errorf("StateDir = %q, want %q", cfg.StateDir, want)
			}
			if cfg.EphemeralState != tc.wantEphemeral {
				t.Errorf("EphemeralState = %v, want %v", cfg.EphemeralState, tc.wantEphemeral)
			}
		})
	}
}

// Explicit absolute overrides (env / yaml) must be preserved verbatim.
func TestStateDirAbsoluteOverridePreserved(t *testing.T) {
	t.Run("env override", func(t *testing.T) {
		restoreStateEnv(t)
		envDir := filepath.Join(t.TempDir(), "custom-state")
		t.Setenv("TS_STATE_DIR", envDir)
		cfg, err := Merge(PartialConfig{}, baseFlags(FlagSet{ManualMode: true}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.StateDir != envDir {
			t.Errorf("env override must be preserved, got %q", cfg.StateDir)
		}
	})

	t.Run("yaml override", func(t *testing.T) {
		restoreStateEnv(t)
		yamlDir := filepath.Join(t.TempDir(), "yaml-state")
		cfg, err := Merge(PartialConfig{StateDir: yamlDir}, baseFlags(FlagSet{ManualMode: true}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.StateDir != yamlDir {
			t.Errorf("yaml override must be preserved, got %q", cfg.StateDir)
		}
	})
}

// A padded control URL must be normalized (trimmed) and stored, so the value
// validated is the value handed to tsnet (#209 review).
func TestMergeControlURLNormalized(t *testing.T) {
	restoreStateEnv(t)
	cfg, err := Merge(PartialConfig{ControlURL: "  https://hs.example.com  "}, FlagSet{
		Target:  "100.64.0.1:3389",
		AuthKey: "hskey-test123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ControlURL != "https://hs.example.com" {
		t.Errorf("control URL should be trimmed and stored, got %q", cfg.ControlURL)
	}
}

// Control-plane routing by key prefix (#209): a Headscale key (hskey-) needs a
// control URL; a Tailscale key (tskey-) routes to SaaS by default.
func TestMergeControlPlaneForKey(t *testing.T) {
	cases := []struct {
		name       string
		authKey    string
		controlURL string
		wantErr    bool
	}{
		{"headscale key without control url is rejected", "hskey-test123", "", true},
		{"headscale key with control url is accepted", "hskey-test123", "https://hs.example.com", false},
		{"headscale key with whitespace-only control url is rejected", "hskey-test123", "   ", true},
		{"headscale key with http control url is accepted (dev)", "hskey-test123", "http://localhost:8080", false},
		{"tailscale key without control url is accepted (SaaS default)", "tskey-test123", "", false},
		{"tailscale key with custom control url is accepted", "tskey-test123", "https://self-hosted.example.com", false},
		{"control url without scheme is rejected", "tskey-test123", "headscale.example.com", true},
		{"control url with wrong scheme is rejected", "hskey-test123", "ftp://hs.example.com", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreStateEnv(t)
			_, err := Merge(PartialConfig{ControlURL: tc.controlURL}, FlagSet{
				Target:  "100.64.0.1:3389",
				AuthKey: tc.authKey,
			})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q + control %q", tc.authKey, tc.controlURL)
				}
				if !strings.Contains(err.Error(), "control URL") {
					t.Errorf("error should point at the control URL, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
