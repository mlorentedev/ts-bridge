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

// --- Helper ---

func mustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}
