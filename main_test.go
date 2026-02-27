package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		verbose bool
		wantErr bool
		check   func(t *testing.T, cfg Config)
	}{
		{
			name:    "valid config with defaults",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.Target != "100.64.0.1:3389" {
					t.Errorf("expected target 100.64.0.1:3389, got %s", cfg.Target)
				}
				if cfg.LocalAddr != "127.0.0.1:33389" {
					t.Errorf("expected default local addr, got %s", cfg.LocalAddr)
				}
				if cfg.ConnectTimeout != 30*time.Second {
					t.Errorf("expected 30s timeout, got %v", cfg.ConnectTimeout)
				}
			},
		},
		{
			name:    "custom timeout",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123", "TS_TIMEOUT": "1m30s"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.ConnectTimeout != 90*time.Second {
					t.Errorf("expected timeout 1m30s, got %v", cfg.ConnectTimeout)
				}
			},
		},
		{
			name:    "control URL unset defaults to empty",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.ControlURL != "" {
					t.Errorf("expected empty ControlURL, got %q", cfg.ControlURL)
				}
			},
		},
		{
			name:    "control URL set to headscale",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123", "TS_CONTROL_URL": "https://vpn.example.com"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.ControlURL != "https://vpn.example.com" {
					t.Errorf("expected https://vpn.example.com, got %q", cfg.ControlURL)
				}
			},
		},
		{
			name:    "missing target",
			env:     map[string]string{"TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "missing auth key",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389"},
			wantErr: true,
		},
		{
			name:    "invalid auth key format",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "invalid-key-format"},
			wantErr: true,
		},
		{
			name:    "invalid timeout",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123", "TS_TIMEOUT": "invalid"},
			wantErr: true,
		},
		{
			name:    "target no port",
			env:     map[string]string{"TS_TARGET": "100.64.0.1", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "target empty host",
			env:     map[string]string{"TS_TARGET": ":3389", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "target invalid port",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:abc", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "target port too high",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:99999", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "target port zero",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:0", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "target negative port",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:-1", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all config env vars
			for _, key := range []string{"TS_TARGET", "TS_AUTHKEY", "TS_TIMEOUT", "TS_VERBOSE",
				"TS_LOCAL_ADDR", "TS_HOSTNAME", "TS_STATE_DIR", "TS_CONTROL_URL",
				"TS_MAX_CONNECTIONS", "TS_HEALTH_ADDR", "TS_LOG_FORMAT"} {
				os.Unsetenv(key)
			}
			// Set test-specific env vars
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			cfg, err := loadConfig(tt.verbose)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestInitLogger(t *testing.T) {
	oldLogger := logger
	defer func() { logger = oldLogger }()

	tests := []struct {
		name        string
		cfg         Config
		wantHandler string
		wantLevel   slog.Level
	}{
		{
			name:        "default text handler",
			cfg:         Config{LogFormat: "text"},
			wantHandler: "*slog.TextHandler",
			wantLevel:   slog.LevelInfo,
		},
		{
			name:        "json handler",
			cfg:         Config{LogFormat: "json"},
			wantHandler: "*slog.JSONHandler",
			wantLevel:   slog.LevelInfo,
		},
		{
			name:        "verbose enables debug level",
			cfg:         Config{LogFormat: "text", Verbose: true},
			wantHandler: "*slog.TextHandler",
			wantLevel:   slog.LevelDebug,
		},
		{
			name:        "unknown format falls back to text",
			cfg:         Config{LogFormat: "yaml"},
			wantHandler: "*slog.TextHandler",
			wantLevel:   slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initLogger(tt.cfg)

			handlerType := fmt.Sprintf("%T", logger.Handler())
			if handlerType != tt.wantHandler {
				t.Errorf("handler type = %s, want %s", handlerType, tt.wantHandler)
			}

			if !logger.Handler().Enabled(context.Background(), tt.wantLevel) {
				t.Errorf("expected level %v to be enabled", tt.wantLevel)
			}
		})
	}
}

func TestEnvOr(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envValue string
		setEnv   bool
		fallback string
		want     string
	}{
		{"fallback when unset", "TEST_VAR_NOT_SET", "", false, "default", "default"},
		{"env value when set", "TEST_VAR_SET", "custom", true, "default", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(tt.key)
			if tt.setEnv {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}
			if got := envOr(tt.key, tt.fallback); got != tt.want {
				t.Errorf("envOr(%q, %q) = %q, want %q", tt.key, tt.fallback, got, tt.want)
			}
		})
	}
}
