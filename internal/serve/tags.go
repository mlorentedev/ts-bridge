package serve

import "tailscale.com/ipn/ipnstate"

// NodeTags returns the tags the control plane gave the node in status, or none
// when status, its Self or its tags are missing.
func NodeTags(status *ipnstate.Status) []string {
	if status == nil || status.Self == nil || status.Self.Tags == nil {
		return nil
	}
	return status.Self.Tags.AsSlice()
}
