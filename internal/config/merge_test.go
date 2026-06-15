package config

import (
	"os"
	"path/filepath"
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
	flags := FlagSet{Timeout: mustParseDuration("30s")}

	cfg, err := Merge(yaml, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Flag (30s) > Env (1m) > YAML (3m) > Default (30s)
	if cfg.ConnectTimeout != 30*time.Second {
		t.Errorf("expected flag timeout 30s, got %v", cfg.ConnectTimeout)
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

func TestMergeMissingYAMLFileNotError(t *testing.T) {
	// Missing YAML file should not error — config is optional
	cfg, err := LoadYAMLConfig("/nonexistent/path.yaml")
	if err != nil {
		t.Fatalf("missing YAML file should not error, got: %v", err)
	}
	_ = cfg
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
		name            string
		envDialRetries  string
		flagDialRetries int
		wantErr         bool
		wantRetries     int
		errContains     string
	}{
		{
			name:            "negative flag rejected",
			flagDialRetries: -1,
			wantErr:         true,
			errContains:     "dial retries must be non-negative",
		},
		{
			name:            "zero flag accepted",
			flagDialRetries: 0,
			wantErr:         false,
			wantRetries:     0,
		},
		{
			name:            "positive flag accepted",
			flagDialRetries: 5,
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
			name:           "negative env with flag 0 rejected",
			envDialRetries: "-5",
			flagDialRetries: 0,
			wantErr:        true,
			errContains:    "dial retries must be non-negative",
		},
		{
			name:            "flag overrides env (flag 3, env 10)",
			envDialRetries:  "10",
			flagDialRetries: 3,
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

// --- Helper ---

func mustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}
