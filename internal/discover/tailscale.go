package discover

import (
	"context"
	"fmt"
	"strings"
	"time"

	tsv2 "tailscale.com/client/tailscale/v2"
)

// TailscaleClient is the interface for the Tailscale device-list API call.
// Implemented by the v2 client in production and by a mock for testing.
type TailscaleClient interface {
	Devices(ctx context.Context) ([]tsv2.Device, error)
}

// ListDevices fetches all devices from the Tailscale API.
func ListDevices(ctx context.Context, authKey, tailnet string) ([]Device, error) {
	if authKey == "" {
		return nil, fmt.Errorf("auth key is required")
	}
	if tailnet == "" {
		return nil, fmt.Errorf("tailnet is required")
	}

	client := &tsv2.Client{Tailnet: tailnet, APIKey: authKey}
	devices, err := client.Devices().List(ctx)
	if err != nil {
		if hint, remediation := diagnoseTailscaleAPIError(err); hint != "" {
			return nil, fmt.Errorf("%s — %s", err, remediation)
		}
		return nil, fmt.Errorf("fetch devices: %w", err)
	}

	return convertTailscaleDevices(devices), nil
}

// convertTailscaleDevices converts v2 tailscale devices to our Device type.
// Exported for testing.
func convertTailscaleDevices(devices []tsv2.Device) []Device {
	result := make([]Device, 0, len(devices))
	for _, d := range devices {
		result = append(result, Device{
			Addresses:  d.Addresses,
			DeviceID:   d.ID, // v2 renamed DeviceID -> ID
			NodeID:     d.NodeID,
			User:       d.User,
			Name:       d.Name,
			Hostname:   d.Hostname,
			OS:         d.OS,
			Tags:       d.Tags,
			Created:    formatTSTime(d.Created),
			LastSeen:   formatTSTimePtr(d.LastSeen),
			Expires:    formatTSTime(d.Expires),
			Authorized: d.Authorized,
			IsExternal: d.IsExternal,
		})
	}
	return result
}

// formatTSTime renders a v2 API timestamp (which wraps time.Time) as an RFC3339
// string. It preserves the v1 client's behaviour of leaving an unset time empty
// rather than emitting the zero value "0001-01-01T00:00:00Z" — the API sends ""
// for devices with no such date (e.g. the hello service).
func formatTSTime(t tsv2.Time) string {
	if t.Time.IsZero() {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}

// formatTSTimePtr is formatTSTime for the pointer-valued fields. LastSeen is nil
// when the device is currently connected to the control plane; that maps to "".
func formatTSTimePtr(t *tsv2.Time) string {
	if t == nil {
		return ""
	}
	return formatTSTime(*t)
}

// diagnoseTailscaleAPIError inspects a Tailscale API error and returns
// an actionable hint. Returns empty strings for unrecognized errors.
func diagnoseTailscaleAPIError(err error) (hint, remediation string) {
	if err == nil {
		return "", ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "api key does not exist"),
		strings.Contains(msg, "invalid key"),
		strings.Contains(msg, "key expired"),
		strings.Contains(msg, "unauthorized"),
		strings.Contains(msg, "401"),
		strings.Contains(msg, "403"):
		return "auth key rejected by Tailscale API (invalid, expired, or missing devices:read scope)",
			"generate a new API key at https://login.tailscale.com/admin/settings/keys with the \"devices:read\" scope"
	case strings.Contains(msg, "not found"),
		strings.Contains(msg, "404"):
		return "tailnet not found — check that the tailnet name is correct",
			"find your tailnet name at https://login.tailscale.com/admin/dns"
	case strings.Contains(msg, "rate limit"),
		strings.Contains(msg, "429"):
		return "Tailscale API rate limit exceeded",
			"wait a moment and retry; the API allows ~60 requests/minute per key"
	case strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "i/o timeout"),
		strings.Contains(msg, "connection refused"):
		return "Tailscale API unreachable",
			"check network connectivity to https://api.tailscale.com"
	}
	return "", ""
}
