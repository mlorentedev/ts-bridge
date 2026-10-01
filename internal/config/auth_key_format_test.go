package config

import (
	"strings"
	"testing"
)

func TestMergeRejectsNonMachineAndMalformedTailscaleKeys(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "API token", key: "tskey-api-id-secret"},
		{name: "OAuth client secret", key: "tskey-client-id-secret"},
		{name: "missing auth segments", key: "tskey-auth"},
		{name: "missing secret segment", key: "tskey-auth-id"},
		{name: "unknown tskey type", key: "tskey-foo"},
		{name: "concatenated auth keys", key: "tskey-auth-id-secret-auth-id-secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.key
			t.Setenv("TS_AUTHKEY", "")
			t.Setenv("TS_TARGET", "")
			_, err := Merge(PartialConfig{}, FlagSet{
				Target:  "mesh-host:22",
				AuthKey: key,
			})
			if err == nil || !strings.Contains(err.Error(), "auth key invalid format") {
				t.Fatalf("Merge() error = %v, want auth key format error", err)
			}
			if strings.Contains(err.Error(), key) {
				t.Fatal("validation error leaked the credential value")
			}
		})
	}
}
