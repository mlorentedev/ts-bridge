package host

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoadServeFile(t *testing.T) {
	for _, goos := range []string{goosWindows, goosLinux} {
		t.Run(goos, func(t *testing.T) {
			path := writeHostFile(t, "host:\n  services:\n    - kind: ssh\n    - kind: rdp\n      port: 45000\n")
			file, err := LoadServeFile(path, goos, nil)
			if err != nil {
				t.Fatalf("serve must accept already-provisioned SSH/RDP: %v", err)
			}
			services := file.Host.Services
			if len(services) != 2 || services[0].Kind != ServiceSSH || services[0].Port != nil ||
				services[1].Kind != ServiceRDP || services[1].Port == nil || *services[1].Port != 45000 {
				t.Fatalf("declared services were not preserved: %+v", services)
			}
			// Linux converges ssh since #521; Windows still has no step for it.
			_, err = LoadFile(path, goos, nil)
			if goos == goosWindows && (err == nil || !strings.Contains(err.Error(), "#521")) {
				t.Errorf("serve must not enable SSH convergence on windows: LoadFile() = %v", err)
			}
			if goos == goosLinux && err != nil {
				t.Errorf("LoadFile() on linux = %v, want ssh accepted", err)
			}
		})
	}
}

func TestLoadServeFile_Validation(t *testing.T) {
	tests := []struct{ name, goos, body, want string }{
		{"empty list", goosWindows, "host:\n  services: []\n", "host.services: empty"},
		{"unknown kind", goosWindows, "host:\n  services: [{kind: http}]\n", "must be one of"},
		{"duplicate SSH", goosWindows, "host:\n  services: [{kind: ssh}, {kind: ssh}]\n", "declared twice"},
		{"zero port", goosWindows, "host:\n  services: [{kind: ssh, port: 0}]\n", "between 1 and 65535"},
		{"oversized port", goosLinux, "host:\n  services: [{kind: ssh, port: 65536}]\n", "between 1 and 65535"},
		{"Windows VNC", goosWindows, "host:\n  services: [{kind: vnc}]\n", "no vnc step"},
		{"Linux VNC", goosLinux, "host:\n  services: [{kind: vnc}]\n", "no vnc step"},
		{"macOS", goosDarwin, "host:\n  services: [{kind: ssh}]\n", "#459"},
		{"empty control", goosWindows, "host:\n  services: [{kind: ssh}]\n  control_url: \"\"\n", "host.control_url: empty"},
		{"Linux power", goosLinux, "host:\n  services: [{kind: ssh}]\n  power: {prevent_sleep: true}\n", "host.power"},
		{"relative state", goosWindows, "host:\n  mesh: {state_dir: relative}\n", "must be an absolute path"},
		{"exposed health", goosWindows, "host:\n  mesh: {health_addr: '0.0.0.0:9099'}\n", "must be a loopback"},
		{"invalid bootstrap", goosLinux, "host:\n  mesh: {bootstrap_ssh: '-oProxyCommand=x'}\n", "host.mesh.bootstrap_ssh"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadServeFile(writeHostFile(t, tc.body), tc.goos, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("LoadServeFile() = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadServeFile_Warnings(t *testing.T) {
	var warn bytes.Buffer
	body := "host:\n  services: [{kind: ssh}]\n  mesh:\n    hostname: lab\n    extra: true\n"
	if _, err := LoadServeFile(writeHostFile(t, body), goosWindows, &warn); err != nil {
		t.Fatal(err)
	}
	if strings.Count(warn.String(), "WARNING:") != 1 || !strings.Contains(warn.String(), `"host.mesh.extra"`) {
		t.Errorf("warnings = %q, want exactly one naming the unknown mesh key", warn.String())
	}
}
