package host

import "testing"

const prefsCmd = "tailscale debug prefs"

func prefsJSON(controlURL string) string {
	return `{"ControlURL": "` + controlURL + `", "WantRunning": true}`
}

func TestResolveControlPlane(t *testing.T) {
	tests := []struct {
		name     string
		declared string
		runner   *recordingRunner
		want     string
		wantCall bool // whether tailscale was asked
	}{
		{name: "declared wins over detection", declared: "https://hs.example",
			runner: &recordingRunner{output: map[string]string{prefsCmd: prefsJSON("https://other.example")}}, want: "https://hs.example"},
		{name: "declared SaaS URL is saas", declared: "https://controlplane.tailscale.com",
			runner: &recordingRunner{}, want: ControlPlaneSaaS},
		{name: "detected controlplane host is saas", runner: &recordingRunner{output: map[string]string{prefsCmd: prefsJSON("https://controlplane.tailscale.com")}},
			want: ControlPlaneSaaS, wantCall: true},
		{name: "detected login host is saas", runner: &recordingRunner{output: map[string]string{prefsCmd: prefsJSON("https://login.tailscale.com/")}},
			want: ControlPlaneSaaS, wantCall: true},
		{name: "detected control host is saas", runner: &recordingRunner{output: map[string]string{prefsCmd: prefsJSON("https://control.tailscale.com")}},
			want: ControlPlaneSaaS, wantCall: true},
		{name: "detected headscale", runner: &recordingRunner{output: map[string]string{prefsCmd: prefsJSON("https://hs.example:8443")}},
			want: "https://hs.example:8443", wantCall: true},
		{name: "lookalike host is not saas", runner: &recordingRunner{output: map[string]string{prefsCmd: prefsJSON("https://login.tailscale.com.evil.example")}},
			want: "https://login.tailscale.com.evil.example", wantCall: true},
		{name: "empty ControlURL is unknown", runner: &recordingRunner{output: map[string]string{prefsCmd: prefsJSON("")}}, wantCall: true},
		{name: "missing ControlURL is unknown", runner: &recordingRunner{output: map[string]string{prefsCmd: `{"WantRunning": true}`}}, wantCall: true},
		{name: "not JSON is unknown", runner: &recordingRunner{output: map[string]string{prefsCmd: "not json"}}, wantCall: true},
		{name: "not a URL is unknown", runner: &recordingRunner{output: map[string]string{prefsCmd: prefsJSON("hs.example")}}, wantCall: true},
		{name: "tailscale missing is unknown", runner: &recordingRunner{fail: map[string]bool{prefsCmd: true}}, wantCall: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveControlPlane(tt.declared, tt.runner); got != tt.want {
				t.Errorf("resolveControlPlane(%q) = %q, want %q", tt.declared, got, tt.want)
			}
			asked := len(tt.runner.calls) > 0
			if asked != tt.wantCall {
				t.Errorf("tailscale asked = %v, want %v (calls %v)", asked, tt.wantCall, tt.runner.calls)
			}
		})
	}
}

// TestSetupOnAndCheckOn_ControlPlane: the public entry points resolve the
// control plane for every OS, so the descriptor never claims SaaS unasked.
func TestSetupOnAndCheckOn_ControlPlane(t *testing.T) {
	hs := map[string]string{prefsCmd: prefsJSON("https://hs.example")}
	for _, goos := range []string{"linux", "windows", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			setup := setupOn(goos, Config{Port: 3389, FirewallRule: "Rule"}, &recordingRunner{elevated: true, output: hs})
			if setup.ControlURL != "https://hs.example" {
				t.Errorf("setup ControlURL = %q, want the detected one", setup.ControlURL)
			}
			check := checkOn(goos, Config{Port: 3389, FirewallRule: "Rule"}, &recordingRunner{output: hs})
			if check.ControlURL != "https://hs.example" {
				t.Errorf("check ControlURL = %q, want the detected one", check.ControlURL)
			}
		})
	}
}
