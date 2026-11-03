package cmd_test

import (
	"encoding/json"
	"net/http"
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
	upWith := func(node string, published, ready bool) string {
		var status health.TunnelStatus
		status.MarkReady()
		if !ready {
			status.Store(false, "ssh_bootstrap_failed")
		}
		srv := httptest.NewServer(health.Handler(&status, func() (string, []health.ServiceState) {
			return node, []health.ServiceState{{Kind: "rdp", Port: 3390, Published: published}}
		}))
		t.Cleanup(srv.Close)
		return srv.Listener.Addr().String()
	}
	up := func(node string, published bool) string { return upWith(node, published, true) }
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
		{"serve not ready (outage)", upWith("lab-01", true, false), &no},
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

// Plain host check prints no published field, so it must not spend up to 2 s
// probing serve's health endpoint (review of #560).
func TestHostCheckCmd_TextDoesNotProbe(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	dir := isolateHostEnv(t)
	body := "host:\n  services:\n    - kind: rdp\n      port: 3390\n  mesh:\n    hostname: lab-01\n    health_addr: " + srv.Listener.Addr().String() + "\n"
	path := writeTestFile(t, filepath.Join(dir, "host.yaml"), body)
	runHostCmd(t, "check", "windows", []string{"--config", path}, host.CheckResult{RDPPort: 3390})
	if hits != 0 {
		t.Errorf("text check probed serve %d time(s), want none", hits)
	}
}
