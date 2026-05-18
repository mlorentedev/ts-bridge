package config

import (
	"os"
	"strings"
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
				if !cfg.AutoInstance {
					t.Error("expected auto instance mode enabled by default")
				}
				if !strings.HasPrefix(cfg.LocalAddr, "127.0.0.1:") {
					t.Errorf("expected auto local loopback addr, got %s", cfg.LocalAddr)
				}
				if !strings.HasPrefix(cfg.Hostname, "tsb-") {
					t.Errorf("expected auto hostname with tsb- prefix, got %s", cfg.Hostname)
				}
				if !cfg.EphemeralState {
					t.Error("expected default mode to use ephemeral state")
				}
				if !strings.Contains(cfg.StateDir, "ts-bridge") {
					t.Errorf("expected auto state dir under ts-bridge path, got %s", cfg.StateDir)
				}
				if cfg.ConnectTimeout != 30*time.Second {
					t.Errorf("expected 30s timeout, got %v", cfg.ConnectTimeout)
				}
			},
		},
		{
			name: "manual mode restores legacy defaults",
			env: map[string]string{
				"TS_TARGET":      "100.64.0.1:3389",
				"TS_AUTHKEY":     "tskey-auth-test123",
				"TS_MANUAL_MODE": "true",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.AutoInstance {
					t.Error("expected manual mode to disable auto mode")
				}
				if cfg.LocalAddr != defaultLocalAddr {
					t.Errorf("expected legacy local addr, got %s", cfg.LocalAddr)
				}
				if cfg.Hostname != "ts-bridge" {
					t.Errorf("expected legacy hostname, got %s", cfg.Hostname)
				}
				if cfg.StateDir != "./ts-state" {
					t.Errorf("expected legacy state dir, got %s", cfg.StateDir)
				}
				if cfg.EphemeralState {
					t.Error("expected legacy mode to keep persistent state")
				}
			},
		},
		{
			name: "explicit auto flag false restores legacy defaults",
			env: map[string]string{
				"TS_TARGET":        "100.64.0.1:3389",
				"TS_AUTHKEY":       "tskey-auth-test123",
				"TS_AUTO_INSTANCE": "false",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.AutoInstance {
					t.Error("expected explicit false auto flag to disable auto mode")
				}
				if cfg.LocalAddr != defaultLocalAddr {
					t.Errorf("expected legacy local addr, got %s", cfg.LocalAddr)
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
			name: "auto instance mode derives runtime values",
			env: map[string]string{
				"TS_TARGET":        "100.64.0.1:3389",
				"TS_AUTHKEY":       "tskey-auth-test123",
				"TS_AUTO_INSTANCE": "true",
				"TS_INSTANCE_NAME": "office-laptop",
				"TS_PORT_RANGE":    "61000-61100",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if !cfg.AutoInstance {
					t.Error("expected auto instance mode to be enabled")
				}
				if !strings.HasPrefix(cfg.LocalAddr, "127.0.0.1:") {
					t.Errorf("expected loopback local addr, got %s", cfg.LocalAddr)
				}
				if !strings.HasPrefix(cfg.Hostname, "tsb-") {
					t.Errorf("expected generated hostname with tsb- prefix, got %s", cfg.Hostname)
				}
				if !cfg.EphemeralState {
					t.Error("expected generated state directory to be ephemeral")
				}
				if !strings.Contains(cfg.StateDir, "ts-bridge") {
					t.Errorf("expected state dir to include ts-bridge, got %s", cfg.StateDir)
				}
			},
		},
		{
			name: "auto instance mode keeps explicit values",
			env: map[string]string{
				"TS_TARGET":        "100.64.0.1:3389",
				"TS_AUTHKEY":       "tskey-auth-test123",
				"TS_AUTO_INSTANCE": "1",
				"TS_LOCAL_ADDR":    "127.0.0.1:40001",
				"TS_HOSTNAME":      "manual-host",
				"TS_STATE_DIR":     "./custom-state",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.LocalAddr != "127.0.0.1:40001" {
					t.Errorf("expected explicit local addr, got %s", cfg.LocalAddr)
				}
				if cfg.Hostname != "manual-host" {
					t.Errorf("expected explicit hostname, got %s", cfg.Hostname)
				}
				if cfg.StateDir != "./custom-state" {
					t.Errorf("expected explicit state dir, got %s", cfg.StateDir)
				}
				if cfg.EphemeralState {
					t.Error("expected explicit state dir to disable ephemeral cleanup")
				}
			},
		},
		{
			name: "auto instance mode invalid port range",
			env: map[string]string{
				"TS_TARGET":        "100.64.0.1:3389",
				"TS_AUTHKEY":       "tskey-auth-test123",
				"TS_AUTO_INSTANCE": "true",
				"TS_PORT_RANGE":    "bad-range",
			},
			wantErr: true,
		},
		{
			name: "manual mode overrides explicit auto mode flag",
			env: map[string]string{
				"TS_TARGET":        "100.64.0.1:3389",
				"TS_AUTHKEY":       "tskey-auth-test123",
				"TS_AUTO_INSTANCE": "true",
				"TS_MANUAL_MODE":   "true",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.AutoInstance {
					t.Error("expected manual mode to take precedence over auto flag")
				}
				if cfg.LocalAddr != defaultLocalAddr {
					t.Errorf("expected legacy local addr, got %s", cfg.LocalAddr)
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
			name:    "headscale auth key accepted",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "hskey-auth-test123"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.AuthKey != "hskey-auth-test123" {
					t.Errorf("expected hskey-auth-test123, got %q", cfg.AuthKey)
				}
			},
		},
		{
			name:    "invalid timeout",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123", "TS_TIMEOUT": "invalid"},
			wantErr: true,
		},
		{
			name:    "idle timeout defaults to disabled",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.IdleTimeout != 0 {
					t.Errorf("expected IdleTimeout disabled (0), got %v", cfg.IdleTimeout)
				}
			},
		},
		{
			name:    "idle timeout parsed",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123", "TS_IDLE_TIMEOUT": "5m"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.IdleTimeout != 5*time.Minute {
					t.Errorf("expected IdleTimeout 5m, got %v", cfg.IdleTimeout)
				}
			},
		},
		{
			name:    "idle timeout invalid",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123", "TS_IDLE_TIMEOUT": "garbage"},
			wantErr: true,
		},
		{
			name:    "idle timeout negative rejected",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123", "TS_IDLE_TIMEOUT": "-1m"},
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
			for _, key := range []string{"TS_TARGET", "TS_AUTHKEY", "TS_TIMEOUT", "TS_VERBOSE",
				"TS_LOCAL_ADDR", "TS_HOSTNAME", "TS_STATE_DIR", "TS_CONTROL_URL",
				"TS_MAX_CONNECTIONS", "TS_HEALTH_ADDR", "TS_LOG_FORMAT",
				"TS_AUTO_INSTANCE", "TS_INSTANCE_NAME", "TS_PORT_RANGE", "TS_MANUAL_MODE",
				"TS_DRAIN_TIMEOUT", "TS_IDLE_TIMEOUT"} {
				os.Unsetenv(key)
			}
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			cfg, err := LoadConfig(tt.verbose)

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
			if got := EnvOr(tt.key, tt.fallback); got != tt.want {
				t.Errorf("EnvOr(%q, %q) = %q, want %q", tt.key, tt.fallback, got, tt.want)
			}
		})
	}
}
