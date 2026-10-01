package config

import "testing"

func TestPrecedenceProfileAuthKeyIsBelowEnvironmentAndFlags(t *testing.T) {
	t.Run("profile supplies missing key", func(t *testing.T) {
		cfg := mergePrecedence(t, PartialConfig{}, map[string]string{}, FlagSet{
			ProfileAuthKey: "tskey-auth-from-profile",
		})
		if cfg.AuthKey != "tskey-auth-from-profile" {
			t.Fatalf("AuthKey = %q, want profile key", cfg.AuthKey)
		}
	})

	t.Run("environment overrides profile", func(t *testing.T) {
		cfg := mergePrecedence(t, PartialConfig{}, map[string]string{
			"TS_AUTHKEY": "tskey-auth-from-env",
		}, FlagSet{ProfileAuthKey: "tskey-auth-from-profile"})
		if cfg.AuthKey != "tskey-auth-from-env" {
			t.Fatalf("AuthKey = %q, want environment key", cfg.AuthKey)
		}
	})

	t.Run("flag overrides environment and profile", func(t *testing.T) {
		cfg := mergePrecedence(t, PartialConfig{}, map[string]string{
			"TS_AUTHKEY": "tskey-auth-from-env",
		}, FlagSet{
			AuthKey:        "tskey-auth-from-flag",
			ProfileAuthKey: "tskey-auth-from-profile",
		})
		if cfg.AuthKey != "tskey-auth-from-flag" {
			t.Fatalf("AuthKey = %q, want flag key", cfg.AuthKey)
		}
	})
}
