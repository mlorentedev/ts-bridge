package discover

import (
	"context"
	"fmt"
	"strings"

	ts "tailscale.com/client/tailscale"
)

// TailscaleClient is the interface for Tailscale API calls.
// Implemented by *ts.Client for production, and by a mock for testing.
type TailscaleClient interface {
	Devices(ctx context.Context, fields *ts.DeviceFieldsOpts) ([]*ts.Device, error)
}

// ListDevices fetches all devices from the Tailscale API.
func ListDevices(ctx context.Context, authKey, tailnet string) ([]Device, error) {
	if authKey == "" {
		return nil, fmt.Errorf("auth key is required")
	}
	if tailnet == "" {
		return nil, fmt.Errorf("tailnet is required")
	}

	client := ts.NewClient(tailnet, ts.APIKey(authKey))
	devices, err := client.Devices(ctx, nil)
	if err != nil {
		if hint, remediation := diagnoseTailscaleAPIError(err); hint != "" {
			return nil, fmt.Errorf("%s — %s", err, remediation)
		}
		return nil, fmt.Errorf("fetch devices: %w", err)
	}

	return convertTailscaleDevices(devices), nil
}

// convertTailscaleDevices converts ts.Device slices to our Device type.
// Exported for testing.
func convertTailscaleDevices(devices []*ts.Device) []Device {
	result := make([]Device, 0, len(devices))
	for _, d := range devices {
		result = append(result, Device{
			Addresses:  d.Addresses,
			DeviceID:   d.DeviceID,
			NodeID:     d.NodeID,
			User:       d.User,
			Name:       d.Name,
			Hostname:   d.Hostname,
			OS:         d.OS,
			Tags:       d.Tags,
			Created:    d.Created,
			LastSeen:   d.LastSeen,
			Expires:    d.Expires,
			Authorized: d.Authorized,
			IsExternal: d.IsExternal,
		})
	}
	return result
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
