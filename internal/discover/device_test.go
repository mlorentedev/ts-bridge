package discover

import (
	"encoding/json"
	"testing"
)

func TestDeviceJSONUnmarshal_Tailscale(t *testing.T) {
	// Real Tailscale API v2 response shape (minified).
	const jsonInput = `{
		"addresses": ["100.100.99.81/32"],
		"id": "device-id-123",
		"nodeId": "node-id-456",
		"user": "user:alice@example.com",
		"name": "acemagic-lab-1.tail-scale.ts.net.",
		"hostname": "acemagic-lab-1",
		"clientVersion": "1.80.0",
		"updateAvailable": false,
		"os": "linux",
		"tags": ["tag:server"],
		"created": "2026-01-15T10:00:00Z",
		"lastSeen": "2026-06-18T14:30:00Z",
		"keyExpiryDisabled": true,
		"expires": "0001-01-01T00:00:00Z",
		"authorized": true,
		"isExternal": false,
		"blocksIncomingConnections": false,
		"enabledRoutes": [],
		"advertisedRoutes": []
	}`

	var d Device
	if err := json.Unmarshal([]byte(jsonInput), &d); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Core fields.
	if d.Hostname != "acemagic-lab-1" {
		t.Errorf("hostname = %q, want %q", d.Hostname, "acemagic-lab-1")
	}
	if d.Name != "acemagic-lab-1.tail-scale.ts.net." {
		t.Errorf("name = %q, want %q", d.Name, "acemagic-lab-1.tail-scale.ts.net.")
	}
	if len(d.Addresses) != 1 || d.Addresses[0] != "100.100.99.81/32" {
		t.Errorf("addresses = %v, want [100.100.99.81/32]", d.Addresses)
	}
	if !d.Authorized {
		t.Error("authorized should be true")
	}
	if d.OS != "linux" {
		t.Errorf("os = %q, want %q", d.OS, "linux")
	}
	if len(d.Tags) != 1 || d.Tags[0] != "tag:server" {
		t.Errorf("tags = %v, want [tag:server]", d.Tags)
	}
}

func TestDeviceJSONUnmarshal_EmptyFields(t *testing.T) {
	// Minimal response — some fields are empty for external devices.
	const jsonInput = `{
		"addresses": ["100.200.50.12/32"],
		"id": "ext-device-789",
		"name": "external-host.ts.net.",
		"authorized": false,
		"isExternal": true
	}`

	var d Device
	if err := json.Unmarshal([]byte(jsonInput), &d); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if d.Hostname != "" {
		t.Errorf("hostname should be empty for external device, got %q", d.Hostname)
	}
	if !d.IsExternal {
		t.Error("isExternal should be true")
	}
	if d.Authorized {
		t.Error("authorized should be false")
	}
	if d.OS != "" {
		t.Errorf("os should be empty for external device, got %q", d.OS)
	}
}

func TestHeadscaleDeviceJSONUnmarshal(t *testing.T) {
	// Real Headscale API v1 /api/v1/device response shape.
	const jsonInput = `{
		"id": "device-123",
		"name": "headscale-host",
		"user": "alice",
		"ip": "100.64.0.1",
		"lastSeen": "2026-06-18T14:30:00Z",
		"expires": "2027-01-01T00:00:00Z",
		"authorized": true,
		"isExternal": false
	}`

	var d HeadscaleDevice
	if err := json.Unmarshal([]byte(jsonInput), &d); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if d.Name != "headscale-host" {
		t.Errorf("name = %q, want %q", d.Name, "headscale-host")
	}
	if len(d.IPs) != 1 || d.IPs[0] != "100.64.0.1" {
		t.Errorf("ips = %v, want [100.64.0.1]", d.IPs)
	}
	if !d.Authorized {
		t.Error("authorized should be true")
	}
	if d.User != "alice" {
		t.Errorf("user = %q, want %q", d.User, "alice")
	}
}

func TestHeadscaleDeviceJSONUnmarshal_MultipleIPs(t *testing.T) {
	// Headscale can return multiple IPs.
	const jsonInput = `{
		"id": "device-456",
		"name": "multi-ip-host",
		"user": "bob",
		"ip": ["100.64.0.1", "100.64.0.2"],
		"authorized": true
	}`

	var d HeadscaleDevice
	if err := json.Unmarshal([]byte(jsonInput), &d); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(d.IPs) != 2 {
		t.Errorf("expected 2 IPs, got %d", len(d.IPs))
	}
}

func TestDeviceJSONUnmarshal_MultipleAddresses(t *testing.T) {
	const jsonInput = `{
		"addresses": ["100.64.0.1/32", "100.64.0.2/32"],
		"name": "multi-ip-host.ts.net.",
		"authorized": true
	}`

	var d Device
	if err := json.Unmarshal([]byte(jsonInput), &d); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(d.Addresses) != 2 {
		t.Errorf("expected 2 addresses, got %d", len(d.Addresses))
	}
}
