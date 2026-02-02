package main

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig_Valid(t *testing.T) {
	// Setup
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-test123")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	cfg, err := loadConfig(false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Target != "100.64.0.1:3389" {
		t.Errorf("expected target 100.64.0.1:3389, got %s", cfg.Target)
	}
	if cfg.LocalAddr != "127.0.0.1:33389" {
		t.Errorf("expected default local addr, got %s", cfg.LocalAddr)
	}
	if cfg.ConnectTimeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", cfg.ConnectTimeout)
	}
}

func TestLoadConfig_MissingTarget(t *testing.T) {
	os.Unsetenv("TS_TARGET")
	os.Setenv("TS_AUTHKEY", "tskey-auth-test123")
	defer os.Unsetenv("TS_AUTHKEY")

	_, err := loadConfig(false)
	if err == nil {
		t.Fatal("expected error for missing TS_TARGET")
	}
}

func TestLoadConfig_MissingAuthKey(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Unsetenv("TS_AUTHKEY")
	defer os.Unsetenv("TS_TARGET")

	_, err := loadConfig(false)
	if err == nil {
		t.Fatal("expected error for missing TS_AUTHKEY")
	}
}

func TestLoadConfig_InvalidTargetFormat(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{"no port", "100.64.0.1"},
		{"empty host", ":3389"},
		{"invalid port", "100.64.0.1:abc"},
		{"port too high", "100.64.0.1:99999"},
		{"port zero", "100.64.0.1:0"},
		{"negative port", "100.64.0.1:-1"},
	}

	os.Setenv("TS_AUTHKEY", "tskey-auth-test123")
	defer os.Unsetenv("TS_AUTHKEY")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TS_TARGET", tt.target)
			defer os.Unsetenv("TS_TARGET")

			_, err := loadConfig(false)
			if err == nil {
				t.Errorf("expected error for invalid target %q", tt.target)
			}
		})
	}
}

func TestLoadConfig_InvalidAuthKey(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "invalid-key-format")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	_, err := loadConfig(false)
	if err == nil {
		t.Fatal("expected error for invalid auth key format")
	}
}

func TestLoadConfig_CustomTimeout(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-test123")
	os.Setenv("TS_TIMEOUT", "1m30s")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")
	defer os.Unsetenv("TS_TIMEOUT")

	cfg, err := loadConfig(false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := 90 * time.Second
	if cfg.ConnectTimeout != expected {
		t.Errorf("expected timeout %v, got %v", expected, cfg.ConnectTimeout)
	}
}

func TestLoadConfig_InvalidTimeout(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-test123")
	os.Setenv("TS_TIMEOUT", "invalid")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")
	defer os.Unsetenv("TS_TIMEOUT")

	_, err := loadConfig(false)
	if err == nil {
		t.Fatal("expected error for invalid timeout format")
	}
}

func TestEnvOr(t *testing.T) {
	// Test fallback
	os.Unsetenv("TEST_VAR_NOT_SET")
	if got := envOr("TEST_VAR_NOT_SET", "default"); got != "default" {
		t.Errorf("expected default, got %s", got)
	}

	// Test env value
	os.Setenv("TEST_VAR_SET", "custom")
	defer os.Unsetenv("TEST_VAR_SET")
	if got := envOr("TEST_VAR_SET", "default"); got != "custom" {
		t.Errorf("expected custom, got %s", got)
	}
}
