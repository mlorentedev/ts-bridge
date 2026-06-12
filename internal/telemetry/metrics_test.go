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

func TestGetMetrics_ReturnsSnapshot(t *testing.T) {
	ResetMetrics()

	AddTotalConnection()
	AddBytesTx(1024)
	AddBytesRx(2048)
	AddError()
	AddRejectedConn()
	AddActiveConnection(3)

	m := GetMetrics()

	if m.TotalConnections != 1 {
		t.Errorf("TotalConnections = %d, want 1", m.TotalConnections)
	}
	if m.TotalBytesTx != 1024 {
		t.Errorf("TotalBytesTx = %d, want 1024", m.TotalBytesTx)
	}
	if m.TotalBytesRx != 2048 {
		t.Errorf("TotalBytesRx = %d, want 2048", m.TotalBytesRx)
	}
	if m.TotalErrors != 1 {
		t.Errorf("TotalErrors = %d, want 1", m.TotalErrors)
	}
	if m.RejectedConns != 1 {
		t.Errorf("RejectedConns = %d, want 1", m.RejectedConns)
	}
	if m.ActiveConnections != 3 {
		t.Errorf("ActiveConnections = %d, want 3", m.ActiveConnections)
	}
}

func TestGetMetrics_IsolatedFromMutations(t *testing.T) {
	ResetMetrics()

	AddActiveConnection(5)
	snap := GetMetrics()

	// Mutate the global after taking the snapshot.
	ResetMetrics()

	// Snapshot should still reflect the old state.
	if snap.ActiveConnections != 5 {
		t.Errorf("snapshot ActiveConnections = %d, want 5 (mutated after snapshot)", snap.ActiveConnections)
	}
}

func TestAddTotalConnection_Concurrent(t *testing.T) {
	ResetMetrics()

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				AddTotalConnection()
			}
		}()
	}
	wg.Wait()

	expected := int64(goroutines * iterations)
	m := GetMetrics()
	if m.TotalConnections != expected {
		t.Errorf("TotalConnections = %d, want %d", m.TotalConnections, expected)
	}
}

func TestAddBytes_Concurrent(t *testing.T) {
	ResetMetrics()

	const goroutines = 20
	const bytesPerOp = 512
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				AddBytesTx(bytesPerOp)
				AddBytesRx(bytesPerOp)
			}
		}()
	}
	wg.Wait()

	expected := int64(goroutines * opsPerGoroutine * bytesPerOp)
	m := GetMetrics()
	if m.TotalBytesTx != expected {
		t.Errorf("TotalBytesTx = %d, want %d", m.TotalBytesTx, expected)
	}
	if m.TotalBytesRx != expected {
		t.Errorf("TotalBytesRx = %d, want %d", m.TotalBytesRx, expected)
	}
}

func TestAddError_CountsCorrectly(t *testing.T) {
	ResetMetrics()

	for i := 0; i < 7; i++ {
		AddError()
	}

	m := GetMetrics()
	if m.TotalErrors != 7 {
		t.Errorf("TotalErrors = %d, want 7", m.TotalErrors)
	}
}

func TestAddRejectedConn_CountsCorrectly(t *testing.T) {
	ResetMetrics()

	for i := 0; i < 3; i++ {
		AddRejectedConn()
	}

	m := GetMetrics()
	if m.RejectedConns != 3 {
		t.Errorf("RejectedConns = %d, want 3", m.RejectedConns)
	}
}

func TestGetMetrics_EmptyAfterReset(t *testing.T) {
	ResetMetrics()

	AddTotalConnection()
	AddBytesTx(100)
	ResetMetrics()

	m := GetMetrics()
	if m.TotalConnections != 0 {
		t.Errorf("TotalConnections = %d after reset, want 0", m.TotalConnections)
	}
	if m.TotalBytesTx != 0 {
		t.Errorf("TotalBytesTx = %d after reset, want 0", m.TotalBytesTx)
	}
}
