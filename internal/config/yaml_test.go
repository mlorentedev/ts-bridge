package config

import (
	"os"
	"path/filepath"
	"testing"
)

// BUG-016: YAML warnings should use structured logger, not stderr.

func TestDecodeYAML_UnknownFieldWithLogger(t *testing.T) {
	// Set up a logger that captures output.
	SetLogger(nil) // Start clean

	yamlData := []byte(`
version: 1
target: "100.64.0.1:3389"
unknown_field: "should_warn"
`)

	var cfg PartialConfig
	err := decodeYAML(yamlData, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Target != "100.64.0.1:3389" {
		t.Errorf("expected target, got %s", cfg.Target)
	}
}

func TestDecodeYAML_UnknownFieldNoLogger(t *testing.T) {
	// With no logger set, warnings should fall back to stderr (not panic).
	SetLogger(nil)

	yamlData := []byte(`
version: 1
target: "100.64.0.1:3389"
weird_field: "test"
`)

	var cfg PartialConfig
	err := decodeYAML(yamlData, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadYAMLConfig_PermissionWarningWithLogger(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping: running as root, permissions not enforced")
	}

	SetLogger(nil) // No logger — should fall back to stderr

	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
version: 1
target: "100.64.0.1:3389"
`), 0600); err != nil {
		t.Fatal(err)
	}
	// Make world-readable.
	if err := os.Chmod(yamlPath, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadYAMLConfig(yamlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetLogger_AfterLoadYAMLConfig(t *testing.T) {
	// Simulate the real flow: LoadYAMLConfig (no logger) → LoggerInit → more YAML reads.
	SetLogger(nil)

	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
version: 1
target: "100.64.0.1:3389"
`), 0600); err != nil {
		t.Fatal(err)
	}

	// First load (no logger — falls back to stderr).
	_, err := LoadYAMLConfig(yamlPath)
	if err != nil {
		t.Fatalf("first load failed: %v", err)
	}

	// Now set the logger (simulating LoggerInit).
	SetLogger(nil)

	// Second load should use structured logging.
	_, err = LoadYAMLConfig(yamlPath)
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}
}

func TestWarnUnknownField_WithLogger(t *testing.T) {
	// With logger set, warnUnknownField should not panic.
	SetLogger(nil)
	warnUnknownField("test_field")
}

func TestWarnPermission_WithLogger(t *testing.T) {
	SetLogger(nil)
	warnPermission("/tmp/test.yaml", 0644)
}
