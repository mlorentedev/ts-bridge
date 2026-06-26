package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/discover"
)

func TestFilterDevices(t *testing.T) {
	devices := []discover.Device{
		{Hostname: "acemagic-lab-1", Addresses: []string{"100.100.99.81/32"}, Authorized: true},
		{Hostname: "windows-desktop", Addresses: []string{"100.200.50.12/32"}, Authorized: true},
		{Hostname: "macbook-pro", Addresses: []string{"100.100.45.200/32"}, Authorized: false},
		{Hostname: "headscale-server", Addresses: []string{"100.64.0.1/32"}, Authorized: true},
	}

	tests := []struct {
		filter string
		want   []string // expected hostnames
	}{
		{"desktop", []string{"windows-desktop"}},
		{"100.64", []string{"headscale-server"}},
		{"lab", []string{"acemagic-lab-1"}},
		{"mac", []string{"macbook-pro"}},
		{"nonexistent", nil},
		{"", []string{"acemagic-lab-1", "windows-desktop", "macbook-pro", "headscale-server"}},
	}

	for _, tt := range tests {
		t.Run(tt.filter, func(t *testing.T) {
			result := filterDevices(devices, tt.filter)
			if tt.want == nil && len(result) != 0 {
				t.Errorf("filter(%q) = %d devices, want 0", tt.filter, len(result))
				return
			}
			if len(result) != len(tt.want) {
				t.Errorf("filter(%q) = %d devices, want %d", tt.filter, len(result), len(tt.want))
				for _, d := range result {
					t.Logf("  got: %s", d.Hostname)
				}
				return
			}
			for i, d := range result {
				if d.Hostname != tt.want[i] {
					t.Errorf("result[%d].Hostname = %q, want %q", i, d.Hostname, tt.want[i])
				}
			}
		})
	}
}

func TestConvertHeadscaleDevices(t *testing.T) {
	hsDevices := []discover.HeadscaleDevice{
		{Name: "host1", IPs: []string{"100.64.0.1"}, User: "alice", Authorized: true},
		{Name: "host2", IPs: []string{"100.64.0.2", "100.64.0.3"}, User: "bob", Authorized: false},
	}

	result := convertHeadscaleDevices(hsDevices)
	if len(result) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(result))
	}

	if result[0].Hostname != "host1" {
		t.Errorf("device[0].Hostname = %q, want %q", result[0].Hostname, "host1")
	}
	if result[0].User != "alice" {
		t.Errorf("device[0].User = %q, want %q", result[0].User, "alice")
	}
	if len(result[1].Addresses) != 2 {
		t.Errorf("device[1] has %d addresses, want 2", len(result[1].Addresses))
	}
}

func TestDiscoverCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "discover" {
			found = true
			break
		}
	}
	if !found {
		t.Error("discover subcommand not registered with root command")
	}
}

// TestUpdateEnvFile_UsesNonDefaultPort verifies that a non-3389 port is written
// to .env without substituting the default — this is the "port capture" guarantee.
func TestUpdateEnvFile_UsesNonDefaultPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := updateEnvFile(path, "acemagic-office", 45000, ""); err != nil {
		t.Fatalf("updateEnvFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "TS_TARGET=acemagic-office:45000") {
		t.Errorf("expected TS_TARGET=acemagic-office:45000, got:\n%s", content)
	}
}

// TestUpdateEnvFile_DefaultPortNotSubstituted verifies port 3389 is also
// captured correctly (no accidental override to another value).
func TestUpdateEnvFile_DefaultPortNotSubstituted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := updateEnvFile(path, "host", 3389, ""); err != nil {
		t.Fatalf("updateEnvFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if !strings.Contains(string(data), "TS_TARGET=host:3389") {
		t.Errorf("expected TS_TARGET=host:3389, got:\n%s", string(data))
	}
}
