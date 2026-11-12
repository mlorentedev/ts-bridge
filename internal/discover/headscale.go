package discover

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ListHeadscaleDevices fetches all devices from a Headscale instance.
func ListHeadscaleDevices(ctx context.Context, apiKey, controlURL string) ([]HeadscaleDevice, error) { //nolint:funlen,gocyclo // baseline #629
	if apiKey == "" {
		return nil, fmt.Errorf("Headscale API key is required")
	}
	if controlURL == "" {
		return nil, fmt.Errorf("Headscale control URL is required")
	}

	// Headscale lists its nodes at /api/v1/node; there is no /api/v1/device (#717).
	apiURL := controlURL
	if !strings.HasSuffix(apiURL, "/") {
		apiURL += "/"
	}
	apiURL += "api/v1/node"

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
			return nil, fmt.Errorf("%w — %s", err, remediation)
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	// A pointer tells {"nodes":[]} (an empty tailnet) from a body with no node
	// list at all, which would otherwise decode to zero nodes and no error.
	var list struct {
		Nodes *[]headscaleNode `json:"nodes"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if list.Nodes == nil {
		return nil, fmt.Errorf("parse response: no \"nodes\" list in the body")
	}
	devices := make([]HeadscaleDevice, 0, len(*list.Nodes))
	for _, n := range *list.Nodes {
		devices = append(devices, n.device())
	}
	return devices, nil
}

// headscaleNode is one entry of Headscale's GET /api/v1/node, as v0.29.7
// returns it (testdata/headscale-v0.29.7-nodes.json).
type headscaleNode struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	GivenName   string   `json:"givenName"`
	IPAddresses []string `json:"ipAddresses"`
	User        struct {
		Name string `json:"name"`
	} `json:"user"`
	LastSeen string  `json:"lastSeen"`
	Expiry   *string `json:"expiry"`
	Online   bool    `json:"online"`
}

// device maps a node. givenName is the node's name in the tailnet (MagicDNS),
// so it wins over the client-reported name. Headscale has no approval step: a
// node it lists is registered, so it is authorized.
func (n headscaleNode) device() HeadscaleDevice {
	d := HeadscaleDevice{ID: n.ID, Name: cmp.Or(n.GivenName, n.Name), User: n.User.Name, IPs: n.IPAddresses,
		LastSeen: n.LastSeen, Authorized: true, Online: n.Online}
	if n.Expiry != nil {
		d.Expires = *n.Expiry
	}
	return d
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
