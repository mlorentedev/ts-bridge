package discover

import (
	"context"
	"testing"

	ts "tailscale.com/client/tailscale"
)

// mockTailscaleClient returns a predefined list of devices.
type mockTailscaleClient struct {
	devices []*ts.Device
	err     error
}

func (m *mockTailscaleClient) Devices(ctx context.Context, fields *ts.DeviceFieldsOpts) ([]*ts.Device, error) {
	return m.devices, m.err
}

func TestListDevices_Success(t *testing.T) {
	mock := &mockTailscaleClient{
		devices: []*ts.Device{
			{
				DeviceID:   "dev-1",
				Hostname:   "desktop",
				Name:       "desktop.tailnet.ts.net.",
				Addresses:  []string{"100.64.0.1/32"},
				Authorized: true,
				OS:         "windows",
				LastSeen:   "2026-06-18T14:00:00Z",
			},
			{
				DeviceID:   "dev-2",
				Hostname:   "server",
				Name:       "server.tailnet.ts.net.",
				Addresses:  []string{"100.64.0.2/32"},
				Authorized: true,
				OS:         "linux",
				LastSeen:   "2026-06-18T13:00:00Z",
			},
		},
	}

	// We can't easily test ListDevices (which creates its own client)
	// without a real API. Instead, test the conversion logic
	// by calling the internal adapter directly.
	result := convertTailscaleDevices(mock.devices)

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
	if result[1].Hostname != "server" {
		t.Errorf("device[1].Hostname = %q, want %q", result[1].Hostname, "server")
	}
}

func TestListDevices_Empty(t *testing.T) {
	mock := &mockTailscaleClient{devices: []*ts.Device{}}
	result := convertTailscaleDevices(mock.devices)
	if len(result) != 0 {
		t.Errorf("expected 0 devices, got %d", len(result))
	}
}

func TestListDevices_ExternalDevice(t *testing.T) {
	mock := &mockTailscaleClient{
		devices: []*ts.Device{
			{
				DeviceID:   "ext-1",
				Hostname:   "external-host",
				Name:       "external.ts.net.",
				Addresses:  []string{"100.200.50.12/32"},
				Authorized: false,
				IsExternal: true,
			},
		},
	}

	result := convertTailscaleDevices(mock.devices)
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
