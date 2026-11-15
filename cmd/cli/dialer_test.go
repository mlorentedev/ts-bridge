package cmd

import (
	"testing"
	"time"

	"ts-bridge/internal/config"
)

// runBridge's dialer must carry the per-attempt timeout; without it a dial
// that hangs spends the whole budget on the first attempt (#805).
func TestNewTargetDialerCarriesTheRetryConfig(t *testing.T) {
	cfg := config.Config{DialTimeout: 5 * time.Second, DialRetries: 3, DialBackoffBase: time.Second, DialBackoffMax: 30 * time.Second}
	d := newTargetDialer(cfg, nil, nil)
	if d.AttemptTimeout != cfg.DialTimeout {
		t.Errorf("AttemptTimeout = %v, want DialTimeout %v", d.AttemptTimeout, cfg.DialTimeout)
	}
	if d.MaxRetries != 3 || d.BaseBackoff != time.Second || d.MaxBackoff != 30*time.Second {
		t.Errorf("retry config = %d/%v/%v, want 3/1s/30s", d.MaxRetries, d.BaseBackoff, d.MaxBackoff)
	}
}
