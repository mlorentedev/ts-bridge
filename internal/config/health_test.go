package config

import (
	"strings"
	"testing"
)

// The health endpoint serves connection counters and the readiness reason, so a
// non-loopback bind is an exposure the operator has to ask for (#633). Names
// are never resolved: only literal loopback IPs and "localhost" count as local.
func TestValidateHealthListener(t *testing.T) {
	tests := []struct {
		name        string
		addr        string
		allowRemote bool
		wantErr     bool
	}{
		{"unset", "", false, false},
		{"loopback IPv4", "127.0.0.1:9090", false, false},
		{"loopback IPv4 range", "127.0.0.2:9090", false, false},
		{"loopback IPv6", "[::1]:9090", false, false},
		{"localhost", "localhost:9090", false, false},
		{"wildcard IPv4", "0.0.0.0:9090", false, true},
		{"wildcard IPv6", "[::]:9090", false, true},
		{"port only", ":9090", false, true},
		{"private address", "10.0.0.5:9090", false, true},
		{"hostname is not resolved", "health.internal:9090", false, true},
		{"localhost lookalike", "localhost.example.com:9090", false, true},
		{"malformed", "garbage", false, true},
		{"wildcard with opt-in", "0.0.0.0:9090", true, false},
		{"hostname with opt-in", "health.internal:9090", true, false},
		{"malformed with opt-in", "garbage", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHealthListener(Config{HealthAddr: tt.addr, HealthAllowRemote: tt.allowRemote})
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateHealthListener(%q) = nil, want a refusal", tt.addr)
			}
			for _, want := range []string{tt.addr, "--health-addr", "--health-allow-remote", "TS_HEALTH_ALLOW_REMOTE"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q should mention %q", err, want)
				}
			}
		})
	}
}

func TestHealthExposed(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"", false},
		{"127.0.0.1:9090", false},
		{"localhost:9090", false},
		{"0.0.0.0:9090", true},
		{"health.internal:9090", true},
		{"garbage", true},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			if got := HealthExposed(Config{HealthAddr: tt.addr}); got != tt.want {
				t.Errorf("HealthExposed(%q) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}

func TestMergeHealthAllowRemote(t *testing.T) {
	tests := []struct {
		name string
		env  string
		flag *bool
		want bool
	}{
		{"unset", "", nil, false},
		{"environment on", "1", nil, true},
		{"environment off", "0", nil, false},
		{"flag on", "", ptr(true), true},
		{"flag off over environment on", "1", ptr(false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mergePrecedence(t, PartialConfig{}, map[string]string{"TS_HEALTH_ALLOW_REMOTE": tt.env}, FlagSet{HealthAllowRemote: tt.flag})
			if cfg.HealthAllowRemote != tt.want {
				t.Errorf("HealthAllowRemote = %v, want %v", cfg.HealthAllowRemote, tt.want)
			}
		})
	}
}
