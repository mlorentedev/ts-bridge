package host

import (
	"encoding/json"
	"net/url"
	"strings"
)

// ControlPlaneSaaS is the ControlURL a result carries for Tailscale SaaS. An
// empty ControlURL means the control plane is unknown, never SaaS: a
// descriptor that claimed SaaS for a Headscale host would send every client
// to the wrong control plane (#453).
const ControlPlaneSaaS = "saas"

// saasHosts are the hostnames Tailscale SaaS answers on: the current control
// plane, the login host, and the legacy default ts-bridge itself uses.
var saasHosts = map[string]bool{
	"controlplane.tailscale.com": true,
	"login.tailscale.com":        true,
	"control.tailscale.com":      true,
}

// resolveControlPlane returns the control plane the host descriptor should
// carry: the declared control_url, else the one the local Tailscale is joined
// to, else "" (unknown). SaaS URLs collapse to ControlPlaneSaaS.
func resolveControlPlane(declared string, r Runner) string {
	if declared == "" {
		declared = detectControlURL(r)
	}
	return classifyControlURL(declared)
}

// detectControlURL reads ControlURL from `tailscale debug prefs`. It returns
// "" when Tailscale is absent, answers something else, or reports an empty
// or malformed URL (an empty one means "the default" only on old clients, and
// guessing SaaS from it is the failure this avoids).
func detectControlURL(r Runner) string {
	out, err := r.Output("tailscale", "debug", "prefs")
	if err != nil {
		return ""
	}
	var prefs struct{ ControlURL string }
	if json.Unmarshal(out, &prefs) != nil {
		return ""
	}
	raw := strings.TrimSpace(prefs.ControlURL)
	if validateControlURL(&raw) != nil {
		return ""
	}
	return raw
}

func classifyControlURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && saasHosts[strings.ToLower(u.Hostname())] {
		return ControlPlaneSaaS
	}
	return raw
}
