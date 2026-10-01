package config

import (
	"strings"
	"testing"
)

func TestMergeRejectsNonMachineAndMalformedTailscaleKeys(t *testing.T) {
	for _, key := range []string{
		"tskey-api-id-secret",
		"tskey-client-id-secret",
		"tskey-auth-id-secret-auth-id-secret",
	} {
		t.Run(key[:strings.Index(key, "-secret")+7], func(t *testing.T) {
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
