package cmd_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	cmdpkg "ts-bridge/cmd/cli"
	"ts-bridge/internal/host"
)

// The ADR-017 §3 contract: the new keys are added and the pre-HOST-004 keys
// stay, so a consumer of either shape keeps working (specs/HOST-004).

func decode(t *testing.T, data []byte, err error) map[string]any {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, data)
	}
	return m
}

func TestCheckJSON_AddsServicesAndSteps(t *testing.T) {
	r := host.CheckResult{
		TailscaleIP: "100.64.0.1", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true,
		Services: []host.Service{{Kind: "rdp", Port: 3389, Enabled: true}},
		Steps: []host.SetupStep{
			{Name: "Tailscale IP", Outcome: host.OutcomeOK, Success: true, Message: "Tailscale IP: 100.64.0.1"},
			{Name: "Firewall rule", Outcome: host.OutcomeChanged, Success: true, Message: "No rule found for port 3389", Fix: "run ts-bridge host setup as an administrator"},
		},
	}
	data, err := cmdpkg.CheckJSONForTest(r)
	m := decode(t, data, err)

	for _, key := range []string{"tailscale_ip", "rdp_port", "rdp_enabled", "firewall_ok", "tailscale_up", "platform", "services", "steps"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
	wantSvc := []any{map[string]any{"kind": "rdp", "port": float64(3389), "enabled": true, "active": nil, "firewall_ok": false, "unread": false}}
	if !reflect.DeepEqual(m["services"], wantSvc) {
		t.Errorf("services = %v, want %v (active is null until a step reads it)", m["services"], wantSvc)
	}
	steps := m["steps"].([]any)
	last := steps[1].(map[string]any)
	if last["outcome"] != "changed" || last["fix"] != "run ts-bridge host setup as an administrator" || last["success"] != true {
		t.Errorf("step = %v, want outcome changed, its fix, and the derived success", last)
	}
}

// An unprivileged check with rdp declared used to report rdp_port 0 and nothing
// else. The declared service and its unread state must be visible in services[].
func TestCheckJSON_ListsADeclaredServiceItCouldNotRead(t *testing.T) {
	r := host.CheckResult{
		Services: []host.Service{{Kind: host.ServiceRDP, Port: 3389, Unread: true}},
		Steps:    []host.SetupStep{{Name: "xrdp service", Outcome: host.OutcomeUnverified, Fix: "sudo ts-bridge host check"}},
	}
	data, err := cmdpkg.CheckJSONForTest(r)
	m := decode(t, data, err)

	if m["rdp_port"] != float64(0) {
		t.Errorf("rdp_port = %v, want the legacy 0 kept", m["rdp_port"])
	}
	svcs, _ := m["services"].([]any)
	if len(svcs) != 1 {
		t.Fatalf("services = %v, want the declared rdp service", m["services"])
	}
	svc := svcs[0].(map[string]any)
	if svc["kind"] != "rdp" || svc["port"] != float64(3389) || svc["unread"] != true {
		t.Errorf("service = %v, want rdp on 3389 with unread true", svc)
	}
	// The zero values are not readings: unread is the only thing that marks them.
	if svc["enabled"] != false || svc["firewall_ok"] != false || svc["active"] != nil {
		t.Errorf("service = %v, want enabled false, firewall_ok false, active null alongside unread", svc)
	}
}

func TestCheckJSON_EmptyListsAreArraysNotNull(t *testing.T) {
	data, err := cmdpkg.CheckJSONForTest(host.CheckResult{})
	m := decode(t, data, err)
	for _, key := range []string{"services", "steps"} {
		if _, ok := m[key].([]any); !ok {
			t.Errorf("%s = %v, want []", key, m[key])
		}
	}
}

func TestSetupJSON_StepsCarryOutcomeAndFix(t *testing.T) {
	data, err := cmdpkg.SetupJSONForTest(host.SetupResult{RDPPort: 3389, Steps: []host.SetupStep{
		{Name: "xrdp service", Outcome: host.OutcomeFailed, Message: "xrdp not installed", Fix: "sudo apt install xrdp"},
	}})
	m := decode(t, data, err)
	step := m["steps"].([]any)[0].(map[string]any)
	want := map[string]any{"name": "xrdp service", "outcome": "failed", "success": false, "message": "xrdp not installed", "fix": "sudo apt install xrdp"}
	if !reflect.DeepEqual(step, want) {
		t.Errorf("step = %v, want %v", step, want)
	}
	if _, ok := m["rdp_port"]; !ok {
		t.Error("rdp_port dropped")
	}
}

// TestSetupJSON_DescriptorControlURL: setup --json carries the same tsb://
// descriptor as setup's text summary and check --json, with a declared
// control_url as its cp= (HOST-005 AC6), and omits it when no Tailscale IP
// or no control plane is known, as the text summary does (#453).
func TestSetupJSON_DescriptorControlURL(t *testing.T) {
	tests := []struct {
		name, ip, controlURL, want string
	}{
		{name: "declared control plane", ip: "100.64.0.1", controlURL: "https://hs.example", want: "cp=https%3A%2F%2Fhs.example"},
		{name: "saas control plane", ip: "100.64.0.1", controlURL: host.ControlPlaneSaaS, want: "cp=saas"},
		{name: "unknown control plane is not saas", ip: "100.64.0.1"},
		{name: "no tailscale ip", controlURL: "https://hs.example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := cmdpkg.SetupJSONOnForTest(tt.ip, host.SetupResult{RDPPort: 3389, ControlURL: tt.controlURL})
			m := decode(t, data, err)
			desc, ok := m["descriptor"].(string)
			if tt.want == "" {
				if ok {
					t.Errorf("descriptor = %q, want the key omitted", desc)
				}
				return
			}
			if !strings.HasPrefix(desc, "tsb://") || !strings.Contains(desc, tt.want) {
				t.Errorf("descriptor = %q, want a tsb:// descriptor with %s", desc, tt.want)
			}
		})
	}
}
