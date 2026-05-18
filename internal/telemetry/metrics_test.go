package telemetry

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestTryClaimConnection_HonorsLimit(t *testing.T) {
	ResetMetrics()

	if !TryClaimConnection(1) {
		t.Fatal("first claim should succeed")
	}
	if TryClaimConnection(1) {
		t.Fatal("second claim should fail when cap=1 and one slot held")
	}

	AddActiveConnection(-1)

	if !TryClaimConnection(1) {
		t.Fatal("claim should succeed again after release")
	}
	AddActiveConnection(-1)
}

func TestTryClaimConnection_AtomicUnderRace(t *testing.T) {
	ResetMetrics()

	const cap = 50
	const attackers = 500

	var success atomic.Int64
	var wg sync.WaitGroup
	wg.Add(attackers)
	start := make(chan struct{})

	for range attackers {
		go func() {
			defer wg.Done()
			<-start
			if TryClaimConnection(cap) {
				success.Add(1)
			}
		}()
	}

	close(start)
	wg.Wait()

	got := success.Load()
	if got != cap {
		t.Fatalf("under race: %d claims succeeded, want exactly %d", got, cap)
	}
	if GetActiveConnections() != cap {
		t.Errorf("ActiveConnections = %d, want %d", GetActiveConnections(), cap)
	}

	// Cleanup so subsequent tests start clean.
	for range cap {
		AddActiveConnection(-1)
	}
}

func TestTryClaimConnection_ZeroCapAlwaysFails(t *testing.T) {
	ResetMetrics()
	if TryClaimConnection(0) {
		t.Fatal("cap=0 should reject all claims")
	}
}
