package config

import (
	"strings"
	"testing"
)

func TestMergeBootstrapSSHPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		yamlSSH  string
		envSSH   string
		flagSSH  string
		yamlAddr string
		envAddr  string
		flagAddr string
		wantSSH  string
		wantAddr string
	}{
		{
			name:     "YAML values",
			yamlSSH:  "yaml-user@yaml-host",
			yamlAddr: "127.0.0.1:1101",
			wantSSH:  "yaml-user@yaml-host",
			wantAddr: "127.0.0.1:1101",
		},
		{
			name:     "environment overrides YAML",
			yamlSSH:  "yaml-user@yaml-host",
			envSSH:   "env-user@env-host",
			yamlAddr: "127.0.0.1:1101",
			envAddr:  "127.0.0.1:1102",
			wantSSH:  "env-user@env-host",
			wantAddr: "127.0.0.1:1102",
		},
		{
			name:     "flags override environment",
			yamlSSH:  "yaml-user@yaml-host",
			envSSH:   "env-user@env-host",
			flagSSH:  "flag-user@flag-host:2222",
			yamlAddr: "127.0.0.1:1101",
			envAddr:  "127.0.0.1:1102",
			flagAddr: "127.0.0.1:1103",
			wantSSH:  "flag-user@flag-host:2222",
			wantAddr: "127.0.0.1:1103",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TS_BOOTSTRAP_SSH", tt.envSSH)
			t.Setenv("TS_BOOTSTRAP_SOCKS_ADDR", tt.envAddr)
			t.Setenv("TS_TARGET", "")
			t.Setenv("TS_AUTHKEY", "")
			t.Setenv("TS_CONTROL_URL", "")

			cfg, err := Merge(
				PartialConfig{
					BootstrapSSH:       tt.yamlSSH,
					BootstrapSOCKSAddr: tt.yamlAddr,
				},
				FlagSet{
					Target:             "mesh-host:22",
					AuthKey:            "hskey-auth-test",
					ControlURL:         "https://vpn.example.com",
					BootstrapSSH:       tt.flagSSH,
					BootstrapSOCKSAddr: tt.flagAddr,
				},
			)
			if err != nil {
				t.Fatalf("Merge() error = %v", err)
			}
			if cfg.BootstrapSSH != tt.wantSSH {
				t.Errorf("BootstrapSSH = %q, want %q", cfg.BootstrapSSH, tt.wantSSH)
			}
			if cfg.BootstrapSOCKSAddr != tt.wantAddr {
				t.Errorf("BootstrapSOCKSAddr = %q, want %q", cfg.BootstrapSOCKSAddr, tt.wantAddr)
			}
		})
	}
}

func TestMergeBootstrapSSHDefaultsToLoopback(t *testing.T) {
	t.Setenv("TS_BOOTSTRAP_SSH", "")
	t.Setenv("TS_BOOTSTRAP_SOCKS_ADDR", "")
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_CONTROL_URL", "")

	cfg, err := Merge(PartialConfig{}, FlagSet{
		Target:       "mesh-host:22",
		AuthKey:      "hskey-auth-test",
		ControlURL:   "https://vpn.example.com",
		BootstrapSSH: "deployer@bastion.example.com",
	})
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	if cfg.BootstrapSOCKSAddr != "127.0.0.1:1055" {
		t.Errorf("BootstrapSOCKSAddr = %q, want 127.0.0.1:1055", cfg.BootstrapSOCKSAddr)
	}
}

func TestMergeRejectsInvalidBootstrapConfig(t *testing.T) {
	tests := []struct {
		name    string
		flags   FlagSet
		wantErr string
	}{
		{
			name: "bootstrap requires custom control URL",
			flags: FlagSet{
				BootstrapSSH: "deployer@bastion.example.com",
			},
			wantErr: "bootstrap SSH requires a custom control URL",
		},
		{
			name: "SOCKS address requires bootstrap",
			flags: FlagSet{
				ControlURL:         "https://vpn.example.com",
				BootstrapSOCKSAddr: "127.0.0.1:1055",
			},
			wantErr: "bootstrap SOCKS address requires bootstrap SSH",
		},
		{
			name: "SOCKS listener must be loopback",
			flags: FlagSet{
				ControlURL:         "https://vpn.example.com",
				BootstrapSSH:       "deployer@bastion.example.com",
				BootstrapSOCKSAddr: "0.0.0.0:1055",
			},
			wantErr: "bootstrap SOCKS listener must bind to loopback",
		},
		{
			name: "SOCKS listener needs a port",
			flags: FlagSet{
				ControlURL:         "https://vpn.example.com",
				BootstrapSSH:       "deployer@bastion.example.com",
				BootstrapSOCKSAddr: "127.0.0.1",
			},
			wantErr: "bootstrap SOCKS listener",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TS_BOOTSTRAP_SSH", "")
			t.Setenv("TS_BOOTSTRAP_SOCKS_ADDR", "")
			t.Setenv("TS_TARGET", "")
			t.Setenv("TS_AUTHKEY", "")
			t.Setenv("TS_CONTROL_URL", "")
			tt.flags.Target = "mesh-host:22"
			tt.flags.AuthKey = "tskey-auth-test"

			_, err := Merge(PartialConfig{}, tt.flags)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Merge() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestMergeRejectsBootstrapListenerCollision(t *testing.T) {
	tests := []struct {
		name    string
		flags   FlagSet
		wantErr string
	}{
		{
			name: "exact local address",
			flags: FlagSet{
				LocalAddr:          "127.0.0.1:1055",
				BootstrapSOCKSAddr: "127.0.0.1:1055",
				ManualMode:         true,
			},
			wantErr: "conflicts with bridge listener",
		},
		{
			name: "equivalent IPv6 address",
			flags: FlagSet{
				LocalAddr:          "[0:0:0:0:0:0:0:1]:1055",
				BootstrapSOCKSAddr: "[::1]:1055",
				ManualMode:         true,
			},
			wantErr: "conflicts with bridge listener",
		},
		{
			name: "auto-derived local address",
			flags: FlagSet{
				BootstrapSOCKSAddr: "127.0.0.1:1055",
				PortRange:          "1055-1055",
			},
			wantErr: "conflicts with bridge listener",
		},
		{
			name: "health listener",
			flags: FlagSet{
				BootstrapSOCKSAddr: "127.0.0.1:1055",
				HealthAddr:         "127.0.0.1:1055",
			},
			wantErr: "conflicts with health listener",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TS_BOOTSTRAP_SSH", "")
			t.Setenv("TS_BOOTSTRAP_SOCKS_ADDR", "")
			t.Setenv("TS_TARGET", "")
			t.Setenv("TS_AUTHKEY", "")
			t.Setenv("TS_CONTROL_URL", "")
			tt.flags.Target = "mesh-host:22"
			tt.flags.AuthKey = "tskey-auth-test"
			tt.flags.ControlURL = "https://vpn.example.com"
			tt.flags.BootstrapSSH = "deployer@bastion.example.com"

			_, err := Merge(PartialConfig{}, tt.flags)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Merge() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeYAMLBootstrapSSH(t *testing.T) {
	var cfg PartialConfig
	err := decodeYAML([]byte(`
version: 1
target: mesh-host:22
control_url: https://vpn.example.com
bootstrap_ssh: deployer@bastion.example.com
bootstrap_socks_addr: 127.0.0.1:1101
`), &cfg)
	if err != nil {
		t.Fatalf("decodeYAML() error = %v", err)
	}
	if cfg.BootstrapSSH != "deployer@bastion.example.com" {
		t.Errorf("BootstrapSSH = %q", cfg.BootstrapSSH)
	}
	if cfg.BootstrapSOCKSAddr != "127.0.0.1:1101" {
		t.Errorf("BootstrapSOCKSAddr = %q", cfg.BootstrapSOCKSAddr)
	}
}
