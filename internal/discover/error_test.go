package discover

import (
	"errors"
	"testing"
)

func TestDiagnoseTailscaleAPIError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantHint    bool
		wantRemed   bool
		hintSubstr  string
		remedSubstr string
	}{
		{
			name:        "invalid auth key",
			err:         errors.New("401 Unauthorized: api key does not exist"),
			wantHint:    true,
			wantRemed:   true,
			hintSubstr:  "auth key rejected",
			remedSubstr: "devices:read",
		},
		{
			name:        "tailnet not found",
			err:         errors.New("404 Not Found: tailnet not found"),
			wantHint:    true,
			wantRemed:   true,
			hintSubstr:  "tailnet not found",
			remedSubstr: "tailnet name",
		},
		{
			name:        "rate limit",
			err:         errors.New("429 Too Many Requests: rate limit exceeded"),
			wantHint:    true,
			wantRemed:   true,
			hintSubstr:  "rate limit",
			remedSubstr: "60 requests",
		},
		{
			name:        "timeout",
			err:         errors.New("context deadline exceeded: i/o timeout"),
			wantHint:    true,
			wantRemed:   true,
			hintSubstr:  "unreachable",
			remedSubstr: "api.tailscale.com",
		},
		{
			name:        "unrecognized error",
			err:         errors.New("some random error"),
			wantHint:    false,
			wantRemed:   false,
			hintSubstr:  "",
			remedSubstr: "",
		},
		{
			name:        "nil error",
			err:         nil,
			wantHint:    false,
			wantRemed:   false,
			hintSubstr:  "",
			remedSubstr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint, remed := diagnoseTailscaleAPIError(tt.err)
			if tt.wantHint && hint == "" {
				t.Errorf("expected hint for error %q", tt.err)
			}
			if tt.wantRemed && remed == "" {
				t.Errorf("expected remediation for error %q", tt.err)
			}
			if tt.hintSubstr != "" && hint != "" && !contains(hint, tt.hintSubstr) {
				t.Errorf("hint %q should contain %q", hint, tt.hintSubstr)
			}
			if tt.remedSubstr != "" && remed != "" && !contains(remed, tt.remedSubstr) {
				t.Errorf("remediation %q should contain %q", remed, tt.remedSubstr)
			}
		})
	}
}

func TestDiagnoseHeadscaleAPIError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantHint    bool
		hintSubstr  string
	}{
		{
			name:       "timeout",
			err:        errors.New("context deadline exceeded: connection refused"),
			wantHint:   true,
			hintSubstr: "unreachable",
		},
		{
			name:       "TLS error",
			err:        errors.New("tls: certificate signed by unknown authority"),
			wantHint:   true,
			hintSubstr: "TLS",
		},
		{
			name:       "unrecognized",
			err:        errors.New("some random error"),
			wantHint:   false,
			hintSubstr: "",
		},
		{
			name:       "nil error",
			err:        nil,
			wantHint:   false,
			hintSubstr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint, remed := diagnoseHeadscaleAPIError(tt.err)
			if tt.wantHint && hint == "" {
				t.Errorf("expected hint for error %q", tt.err)
			}
			if tt.hintSubstr != "" && hint != "" && !contains(hint, tt.hintSubstr) {
				t.Errorf("hint %q should contain %q", hint, tt.hintSubstr)
			}
			// Remediation should also be present for known errors.
			if tt.wantHint && remed == "" {
				t.Errorf("expected remediation for error %q", tt.err)
			}
		})
	}
}
