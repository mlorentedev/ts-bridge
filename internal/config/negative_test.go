package config

import (
	"strings"
	"testing"
	"time"
)

// TS_TIMEOUT and TS_DRAIN_TIMEOUT go through the same validation as every
// other TS_*_TIMEOUT: a value that is not positive warns and is not applied
// (#806). A negative TS_TIMEOUT used to expire every control-plane context.
func TestEnvTimeoutsMustBePositive(t *testing.T) {
	for _, tt := range []struct {
		key  string
		get  func(Config) time.Duration
		want time.Duration
	}{
		{"TS_TIMEOUT", func(c Config) time.Duration { return c.ConnectTimeout }, defaultTimeout},
		{"TS_DRAIN_TIMEOUT", func(c Config) time.Duration { return c.DrainTimeout }, defaultDrainTimeout},
	} {
		for _, v := range []string{"-5s", "0s"} {
			t.Run(tt.key+"="+v, func(t *testing.T) {
				buf := captureWarnings(t)
				cfg := mergePrecedence(t, PartialConfig{}, map[string]string{tt.key: v}, FlagSet{})
				if got := tt.get(cfg); got != tt.want {
					t.Errorf("%s=%s applied %v, want the default %v", tt.key, v, got, tt.want)
				}
				if !strings.Contains(buf.String(), tt.key) {
					t.Errorf("expected a warning naming %s, got: %s", tt.key, buf.String())
				}
			})
		}
	}
}

// A negative value passed as a flag or written in the YAML file is an error,
// as --idle-timeout and --dial-retries already are. It used to be dropped by
// a `> 0` guard and so could not be told apart from "not set" (#806).
func TestMergeRejectsNegativeFlagAndFileValues(t *testing.T) {
	for _, tt := range []struct {
		name  string
		yaml  PartialConfig
		flags FlagSet
	}{
		{"--timeout", PartialConfig{}, FlagSet{Timeout: -time.Second}},
		{"--dial-timeout", PartialConfig{}, FlagSet{DialTimeout: -time.Second}},
		{"--drain-timeout", PartialConfig{}, FlagSet{DrainTimeout: -time.Second}},
		{"--dial-backoff-base", PartialConfig{}, FlagSet{DialBackoffBase: -time.Second}},
		{"--dial-backoff-max", PartialConfig{}, FlagSet{DialBackoffMax: -time.Second}},
		{"--max-conns", PartialConfig{}, FlagSet{MaxConns: -1}},
		{"timeout", PartialConfig{Timeout: -time.Second}, FlagSet{}},
		{"dial_timeout", PartialConfig{DialTimeout: -time.Second}, FlagSet{}},
		{"drain_timeout", PartialConfig{DrainTimeout: -time.Second}, FlagSet{}},
		{"idle_timeout", PartialConfig{IdleTimeout: -time.Second}, FlagSet{}},
		{"dial_retries", PartialConfig{DialRetries: -1}, FlagSet{}},
		{"dial_backoff_base", PartialConfig{DialBackoffBase: -time.Second}, FlagSet{}},
		{"dial_backoff_max", PartialConfig{DialBackoffMax: -time.Second}, FlagSet{}},
		{"max_connections", PartialConfig{MaxConnections: -1}, FlagSet{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TS_AUTHKEY", precedenceAuthKey)
			tt.yaml.Target = precedenceTarget
			_, err := Merge(tt.yaml, tt.flags)
			if err == nil || !strings.Contains(err.Error(), tt.name) {
				t.Fatalf("Merge with a negative %s: err = %v, want an error naming %s", tt.name, err, tt.name)
			}
		})
	}
}
