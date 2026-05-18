package main

import (
	"errors"
	"strings"
	"testing"
)

func TestDiagnoseTailscaleInitError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantHint    bool
		wantContain string
	}{
		{
			name:     "nil error returns empty",
			err:      nil,
			wantHint: false,
		},
		{
			name:        "API key does not exist surfaces key remediation",
			err:         errors.New("tsnet.Up: backend: invalid key: API key does not exist"),
			wantHint:    true,
			wantContain: "expired, revoked",
		},
		{
			name:        "invalid key pattern",
			err:         errors.New("control: invalid key supplied"),
			wantHint:    true,
			wantContain: "expired, revoked",
		},
		{
			name:        "key expired pattern",
			err:         errors.New("auth key expired"),
			wantHint:    true,
			wantContain: "expired, revoked",
		},
		{
			name:        "context deadline exceeded surfaces network remediation",
			err:         errors.New("context deadline exceeded"),
			wantHint:    true,
			wantContain: "control plane unreachable",
		},
		{
			name:        "i/o timeout surfaces network remediation",
			err:         errors.New("dial tcp: i/o timeout"),
			wantHint:    true,
			wantContain: "control plane unreachable",
		},
		{
			name:     "unknown error stays silent",
			err:      errors.New("something completely unrelated"),
			wantHint: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hint, remediation := diagnoseTailscaleInitError(tc.err)
			if tc.wantHint {
				if hint == "" {
					t.Fatalf("expected hint, got empty")
				}
				if remediation == "" {
					t.Fatalf("expected remediation, got empty")
				}
				if tc.wantContain != "" && !strings.Contains(hint, tc.wantContain) {
					t.Fatalf("hint %q does not contain %q", hint, tc.wantContain)
				}
				return
			}
			if hint != "" || remediation != "" {
				t.Fatalf("expected silent output, got hint=%q remediation=%q", hint, remediation)
			}
		})
	}
}

func TestControlURLForError(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty defaults to Tailscale SaaS marker",
			input:    "",
			expected: "https://controlplane.tailscale.com (default)",
		},
		{
			name:     "custom URL is returned as-is",
			input:    "https://vpn.example.com",
			expected: "https://vpn.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := controlURLForError(tc.input); got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

