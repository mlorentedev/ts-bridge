package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/config"
	"ts-bridge/internal/host"
)

// TestHostDescriptor_RoundTrip: the descriptor host setup and host check
// emit imports into a profile with the same control plane (#453).
func TestHostDescriptor_RoundTrip(t *testing.T) {
	tests := []struct {
		name, controlURL, want, token string
	}{
		// SaaS imports as the default (empty ControlURL), so only the token
		// tells "carried cp=saas" from "dropped cp".
		{name: "saas", controlURL: host.ControlPlaneSaaS, token: "cp=saas"},
		{name: "headscale", controlURL: "https://hs.example:8443", want: "https://hs.example:8443", token: "cp=https"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup, err := setupJSON("100.64.0.1", host.SetupResult{RDPPort: 3389, ControlURL: tt.controlURL})
			if err != nil {
				t.Fatal(err)
			}
			check, err := checkJSON(host.CheckResult{TailscaleIP: "100.64.0.1", RDPPort: 3389, ControlURL: tt.controlURL})
			if err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string][]byte{"setup": setup, "check": check} {
				var out struct{ Descriptor string }
				if err := json.Unmarshal(data, &out); err != nil || out.Descriptor == "" {
					t.Fatalf("%s: no descriptor in %s (%v)", name, data, err)
				}
				if !strings.Contains(out.Descriptor, tt.token) {
					t.Errorf("%s: descriptor %q lacks %q", name, out.Descriptor, tt.token)
				}
				store := filepath.Join(t.TempDir(), "profiles.yaml")
				if err := runImportDirect("host", out.Descriptor, store, false); err != nil {
					t.Fatalf("%s: import %q: %v", name, out.Descriptor, err)
				}
				cfg := config.Config{}
				if err := applyProfile(&cfg, "host", store); err != nil {
					t.Fatalf("%s: applyProfile: %v", name, err)
				}
				if cfg.ControlURL != tt.want || cfg.Target != "100.64.0.1:3389" {
					t.Errorf("%s: imported ControlURL %q Target %q, want %q and 100.64.0.1:3389", name, cfg.ControlURL, cfg.Target, tt.want)
				}
			}
		})
	}
}
