package cmd

import (
	"testing"

	"ts-bridge/internal/config"
)

func TestProxyModeAddresses(t *testing.T) {
	tests := []struct {
		name         string
		cfg          config.Config
		wantListener string
		wantTarget   string
	}{
		{
			name:         "static target",
			cfg:          config.Config{LocalAddr: "127.0.0.1:33389", Target: "mesh-host:22"},
			wantListener: "127.0.0.1:33389",
			wantTarget:   "mesh-host:22",
		},
		{
			name:         "dynamic SOCKS5",
			cfg:          config.Config{SOCKS5Addr: "127.0.0.1:1080"},
			wantListener: "127.0.0.1:1080",
			wantTarget:   "dynamic-socks5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := proxyListenerAddr(tt.cfg); got != tt.wantListener {
				t.Fatalf("proxyListenerAddr() = %q, want %q", got, tt.wantListener)
			}
			if got := readyTarget(tt.cfg); got != tt.wantTarget {
				t.Fatalf("readyTarget() = %q, want %q", got, tt.wantTarget)
			}
		})
	}
}
