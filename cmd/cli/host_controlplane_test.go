package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	cmdpkg "ts-bridge/cmd/cli"
	"ts-bridge/internal/host"
)

const descriptorOmitted = "Descriptor omitted: control plane unknown"

// TestHostText_UnknownControlPlane: neither declared nor detected, setup and
// check print no descriptor and say why, rather than claiming SaaS (#453).
func TestHostText_UnknownControlPlane(t *testing.T) {
	var setup, check bytes.Buffer
	cmdpkg.PrintSetupSummaryForTest(&setup, "100.64.0.1", host.SetupResult{RDPPort: 3389})
	cmdpkg.PrintCheckTextForTest(&check, host.CheckResult{TailscaleIP: "100.64.0.1", RDPPort: 3389})
	for name, out := range map[string]string{"setup": setup.String(), "check": check.String()} {
		if strings.Contains(out, "tsb://") {
			t.Errorf("%s: output = %q, want no descriptor", name, out)
		}
		if !strings.Contains(out, descriptorOmitted) || !strings.Contains(out, "host.control_url") {
			t.Errorf("%s: output = %q, want the omission reason naming host.control_url", name, out)
		}
	}
}

// TestHostText_KnownControlPlane: a known control plane prints the descriptor
// and no omission note; a missing Tailscale IP stays silent about the plane.
func TestHostText_KnownControlPlane(t *testing.T) {
	tests := []struct {
		name, ip, controlURL, want string
	}{
		{name: "saas", ip: "100.64.0.1", controlURL: host.ControlPlaneSaaS, want: "cp=saas"},
		{name: "headscale", ip: "100.64.0.1", controlURL: "https://hs.example", want: "cp=https%3A%2F%2Fhs.example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var setup, check bytes.Buffer
			cmdpkg.PrintSetupSummaryForTest(&setup, tt.ip, host.SetupResult{RDPPort: 3389, ControlURL: tt.controlURL})
			cmdpkg.PrintCheckTextForTest(&check, host.CheckResult{TailscaleIP: tt.ip, RDPPort: 3389, ControlURL: tt.controlURL})
			for name, out := range map[string]string{"setup": setup.String(), "check": check.String()} {
				if !strings.Contains(out, tt.want) || strings.Contains(out, descriptorOmitted) {
					t.Errorf("%s: output = %q, want %s and no omission note", name, out, tt.want)
				}
			}
		})
	}
}

func TestHostText_NoIPKeepsOmissionQuiet(t *testing.T) {
	var out bytes.Buffer
	cmdpkg.PrintCheckTextForTest(&out, host.CheckResult{RDPPort: 3389})
	if strings.Contains(out.String(), descriptorOmitted) {
		t.Errorf("output = %q, want no control-plane note without a Tailscale IP", out.String())
	}
}

// TestCheckJSON_UnknownControlPlane: check --json omits the descriptor key.
func TestCheckJSON_UnknownControlPlane(t *testing.T) {
	data, err := cmdpkg.CheckJSONForTest(host.CheckResult{TailscaleIP: "100.64.0.1", RDPPort: 3389})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "descriptor") {
		t.Errorf("check json = %s, want no descriptor key", data)
	}
}
