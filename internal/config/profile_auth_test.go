package config

import "testing"

func TestPrecedenceProfileAuthKeyIsBelowEnvironmentAndFlags(t *testing.T) {
	t.Run("profile supplies missing key", func(t *testing.T) {
		cfg := mergePrecedence(t, PartialConfig{}, map[string]string{}, FlagSet{
			ProfileAuthKey: "tskey-from-profile",
		})
		if cfg.AuthKey != "tskey-from-profile" {
			t.Fatalf("AuthKey = %q, want profile key", cfg.AuthKey)
		}
	})

	t.Run("environment overrides profile", func(t *testing.T) {
		cfg := mergePrecedence(t, PartialConfig{}, map[string]string{
			"TS_AUTHKEY": "tskey-from-env",
		}, FlagSet{ProfileAuthKey: "tskey-from-profile"})
		if cfg.AuthKey != "tskey-from-env" {
			t.Fatalf("AuthKey = %q, want environment key", cfg.AuthKey)
		}
	})

	t.Run("flag overrides environment and profile", func(t *testing.T) {
		cfg := mergePrecedence(t, PartialConfig{}, map[string]string{
			"TS_AUTHKEY": "tskey-from-env",
		}, FlagSet{
			AuthKey:        "tskey-from-flag",
			ProfileAuthKey: "tskey-from-profile",
		})
		if cfg.AuthKey != "tskey-from-flag" {
			t.Fatalf("AuthKey = %q, want flag key", cfg.AuthKey)
		}
	})
}
