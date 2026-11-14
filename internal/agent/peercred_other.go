//go:build !linux && !darwin && !windows

package agent

import "errors"

// peerUID fails closed: a Unix without a supported peer-credential call cannot
// name its callers, and the agent never serves a caller it cannot name.
func peerUID(int) (uint32, error) {
	return 0, errors.New("peer credentials are not available on this platform")
}
