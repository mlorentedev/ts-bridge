package cmd_test

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ts-bridge/internal/health"
	"ts-bridge/internal/host"
)

// TestHostCheckCmd_Published: with host.mesh.health_addr declared, check
// --json reports per declared service whether serve publishes it, trusting
// only a responder that names the file's node (AC6).
func TestHostCheckCmd_Published(t *testing.T) {
	up := func(node string, published bool) string {
		var status health.TunnelStatus
		status.MarkReady()
		srv := httptest.NewServer(health.Handler(&status, func() (string, []health.ServiceState) {
			return node, []health.ServiceState{{Kind: "rdp", Port: 3390, Published: published}}
		}))
		t.Cleanup(srv.Close)
		return srv.Listener.Addr().String()
	}
	down := func() string {
		srv := httptest.NewServer(nil)
		addr := srv.Listener.Addr().String()
		srv.Close()
		return addr
	}
	yes, no := true, false
	tests := []struct {
		name string
		addr string // "" declares no health_addr
		want *bool
	}{
		{"serve publishes it", up("lab-01", true), &yes},
		{"serve does not publish it", up("lab-01", false), &no},
		{"a responder naming another node", up("intruder", true), &no},
		{"serve not running", down(), &no},
		{"no health_addr", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolateHostEnv(t)
			body := "host:\n  services:\n    - kind: rdp\n      port: 3390\n  mesh:\n    hostname: lab-01\n"
			if tc.addr != "" {
				body += "    health_addr: " + tc.addr + "\n"
			}
			path := writeTestFile(t, filepath.Join(dir, "host.yaml"), body)
			ready := host.CheckResult{RDPPort: 3390, Services: []host.Service{{Kind: "rdp", Port: 3390, Enabled: true}}}
			run := runHostCmd(t, "check", "windows", []string{"--json", "--config", path}, ready)
			var out struct {
				Services []struct {
					Kind      string `json:"kind"`
					Published *bool  `json:"published"`
				} `json:"services"`
			}
			if err := json.Unmarshal([]byte(run.stdout), &out); err != nil || len(out.Services) != 1 {
				t.Fatalf("stdout %q: %v", run.stdout, err)
			}
			got := out.Services[0].Published
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Errorf("published = %v, want %v", ptrStr(got), ptrStr(tc.want))
			}
		})
	}
}

func ptrStr(b *bool) string {
	if b == nil {
		return "absent"
	}
	if *b {
		return "true"
	}
	return "false"
}
