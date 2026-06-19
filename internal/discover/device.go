// Package discover provides tailnet device discovery via Tailscale and Headscale APIs.
package discover

import "encoding/json"

// HeadscaleDevice represents a device as returned by the Headscale API v1.
// Headscale's /api/v1/device returns a different shape than Tailscale's API v2.
type HeadscaleDevice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	User string `json:"user"`

	ipRaw any // raw "ip" field, unmarshaled by custom UnmarshalJSON
	IPs      []string

	LastSeen string `json:"lastSeen"`
	Expires  string `json:"expires"`

	Authorized bool `json:"authorized"`
	IsExternal bool `json:"isExternal"`
}

// UnmarshalJSON handles the fact that Headscale returns "ip" as either a
// string (single IP) or an array of strings (multiple IPs).
func (d *HeadscaleDevice) UnmarshalJSON(data []byte) error {
	type alias HeadscaleDevice
	aux := &struct {
		IP any `json:"ip"`
		*alias
	}{
		alias: (*alias)(d),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	d.ipRaw = aux.IP

	switch v := aux.IP.(type) {
	case string:
		if v != "" {
			d.IPs = []string{v}
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				d.IPs = append(d.IPs, s)
			}
		}
	}
	return nil
}

// Device represents a tailnet device as returned by the Tailscale API v2.
type Device struct {
	Addresses []string `json:"addresses"`
	DeviceID  string   `json:"id"`
	NodeID    string   `json:"nodeId"`
	User      string   `json:"user"`
	Name      string   `json:"name"`
	Hostname  string   `json:"hostname"`

	ClientVersion     string   `json:"clientVersion"`
	UpdateAvailable   bool     `json:"updateAvailable"`
	OS                string   `json:"os"`
	Tags              []string `json:"tags"`
	Created           string   `json:"created"`
	LastSeen          string   `json:"lastSeen"`
	KeyExpiryDisabled bool     `json:"keyExpiryDisabled"`
	Expires           string   `json:"expires"`
	Authorized        bool     `json:"authorized"`
	IsExternal        bool     `json:"isExternal"`
	MachineKey        string   `json:"machineKey"`
	NodeKey           string   `json:"nodeKey"`

	BlocksIncomingConnections bool     `json:"blocksIncomingConnections"`
	EnabledRoutes             []string `json:"enabledRoutes"`
	AdvertisedRoutes          []string `json:"advertisedRoutes"`
}
