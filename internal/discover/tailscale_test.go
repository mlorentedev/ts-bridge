package discover

import (
	"testing"
	"time"

	tsv2 "tailscale.com/client/tailscale/v2"
)

// tsTime builds a v2 API timestamp from an RFC3339 string for use in fixtures.
func tsTime(t *testing.T, s string) *tsv2.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return &tsv2.Time{Time: parsed}
}

// These tests target convertTailscaleDevices, the adapter from the v2 API type
// to our Device. ListDevices itself constructs its own *tsv2.Client and would
// need a live API or an HTTP stub, so the conversion — where the field mapping
// bugs actually live — is exercised directly.

func TestConvertTailscaleDevices_MapsFields(t *testing.T) {
	devices := []tsv2.Device{
		{
			ID:         "dev-1",
			Hostname:   "desktop",
			Name:       "desktop.tailnet.ts.net.",
			Addresses:  []string{"100.64.0.1/32"},
			Authorized: true,
			OS:         "windows",
			LastSeen:   tsTime(t, "2026-06-18T14:00:00Z"),
		},
		{
			ID:         "dev-2",
			Hostname:   "server",
			Name:       "server.tailnet.ts.net.",
			Addresses:  []string{"100.64.0.2/32"},
			Authorized: true,
			OS:         "linux",
			LastSeen:   tsTime(t, "2026-06-18T13:00:00Z"),
		},
	}

	result := convertTailscaleDevices(devices)

	if len(result) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(result))
	}

	if result[0].Hostname != "desktop" {
		t.Errorf("device[0].Hostname = %q, want %q", result[0].Hostname, "desktop")
	}
	if result[0].OS != "windows" {
		t.Errorf("device[0].OS = %q, want %q", result[0].OS, "windows")
	}
	if !result[0].Authorized {
		t.Error("device[0] should be authorized")
	}
	// The v2 DeviceID (renamed from ID) and the *Time -> RFC3339 string mapping
	// must round-trip unchanged.
	if result[0].DeviceID != "dev-1" {
		t.Errorf("device[0].DeviceID = %q, want %q", result[0].DeviceID, "dev-1")
	}
	if result[0].LastSeen != "2026-06-18T14:00:00Z" {
		t.Errorf("device[0].LastSeen = %q, want %q", result[0].LastSeen, "2026-06-18T14:00:00Z")
	}
	if result[1].Hostname != "server" {
		t.Errorf("device[1].Hostname = %q, want %q", result[1].Hostname, "server")
	}
}

func TestConvertTailscaleDevices_Empty(t *testing.T) {
	result := convertTailscaleDevices([]tsv2.Device{})
	if len(result) != 0 {
		t.Errorf("expected 0 devices, got %d", len(result))
	}
}

// A device connected to the control plane reports a nil LastSeen; it must map to
// an empty string, not a zero-value timestamp.
func TestConvertTailscaleDevices_ConnectedDeviceNilLastSeen(t *testing.T) {
	devices := []tsv2.Device{
		{ID: "dev-online", Hostname: "online-host", Authorized: true, LastSeen: nil},
	}

	result := convertTailscaleDevices(devices)
	if len(result) != 1 {
		t.Fatalf("expected 1 device, got %d", len(result))
	}
	if result[0].LastSeen != "" {
		t.Errorf("nil LastSeen should map to \"\", got %q", result[0].LastSeen)
	}
}

func TestConvertTailscaleDevices_ExternalDevice(t *testing.T) {
	devices := []tsv2.Device{
		{
			ID:         "ext-1",
			Hostname:   "external-host",
			Name:       "external.ts.net.",
			Addresses:  []string{"100.200.50.12/32"},
			Authorized: false,
			IsExternal: true,
		},
	}

	result := convertTailscaleDevices(devices)
	if len(result) != 1 {
		t.Fatalf("expected 1 device, got %d", len(result))
	}
	if !result[0].IsExternal {
		t.Error("device should be marked as external")
	}
	if result[0].Authorized {
		t.Error("external device should not be authorized")
	}
}
