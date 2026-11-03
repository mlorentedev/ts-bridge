package serve

import (
	"slices"
	"testing"

	"tailscale.com/ipn/ipnstate"
	"tailscale.com/types/views"
)

func TestNodeTags(t *testing.T) {
	tags := views.SliceOf([]string{"tag:lab"})
	tests := []struct {
		name   string
		status *ipnstate.Status
		want   []string
	}{
		{"tagged", &ipnstate.Status{Self: &ipnstate.PeerStatus{Tags: &tags}}, []string{"tag:lab"}},
		{"no tags", &ipnstate.Status{Self: &ipnstate.PeerStatus{}}, nil},
		{"no Self", &ipnstate.Status{}, nil},
		{"no status", nil, nil},
	}
	for _, tc := range tests {
		if got := NodeTags(tc.status); !slices.Equal(got, tc.want) {
			t.Errorf("%s: NodeTags = %v, want %v", tc.name, got, tc.want)
		}
	}
}
