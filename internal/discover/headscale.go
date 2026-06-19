package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ListHeadscaleDevices fetches all devices from a Headscale instance.
func ListHeadscaleDevices(ctx context.Context, apiKey, controlURL string) ([]HeadscaleDevice, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Headscale API key is required")
	}
	if controlURL == "" {
		return nil, fmt.Errorf("Headscale control URL is required")
	}

	// Normalize URL: ensure it ends with /api/v1/device
	apiURL := controlURL
	if !strings.HasSuffix(apiURL, "/") {
		apiURL += "/"
	}
	apiURL += "api/v1/device"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "ts-bridge/discover")

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		if hint, remediation := diagnoseHeadscaleAPIError(err); hint != "" {
			return nil, fmt.Errorf("%s — %s", err, remediation)
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, fmt.Errorf("Headscale API auth failed (status %d): %s — %s",
				resp.StatusCode, msg, "check TS_HEADSCALE_API_KEY is valid and has read permissions")
		case http.StatusNotFound:
			return nil, fmt.Errorf("Headscale API endpoint not found (status %d): %s — %s",
				resp.StatusCode, msg, "verify TS_CONTROL_URL points to a valid Headscale instance")
		default:
			return nil, fmt.Errorf("Headscale API error (status %d): %s",
				resp.StatusCode, msg)
		}
	}

	// Headscale returns either an array or an object with a "devices" key.
	// Read the body once, then try both formats.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var rawDevices []HeadscaleDevice
	if err := json.Unmarshal(body, &rawDevices); err != nil {
		// Try object wrapper: {"devices": [...]}
		var wrapper struct {
			Devices []HeadscaleDevice `json:"devices"`
		}
		if err2 := json.Unmarshal(body, &wrapper); err2 != nil {
			return nil, fmt.Errorf("parse response: unexpected format (tried array and object wrapper): %w", err)
		}
		rawDevices = wrapper.Devices
	}

	return rawDevices, nil
}

// diagnoseHeadscaleAPIError inspects a Headscale API error and returns
// an actionable hint. Returns empty strings for unrecognized errors.
func diagnoseHeadscaleAPIError(err error) (hint, remediation string) {
	if err == nil {
		return "", ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "i/o timeout"),
		strings.Contains(msg, "connection refused"):
		return "Headscale API unreachable",
			"check that TS_CONTROL_URL points to a running Headscale instance and the network is reachable"
	case strings.Contains(msg, "tls"):
		return "TLS certificate error",
			"verify the Headscale instance has a valid TLS certificate or use an HTTPS URL"
	}
	return "", ""
}
