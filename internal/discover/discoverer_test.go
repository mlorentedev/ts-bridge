package discover

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockDiscoverer implements Discoverer for testing.
type mockDiscoverer struct {
	devices []Device
	err     error
	callCount int
	mu      sync.Mutex
}

func (m *mockDiscoverer) List(ctx context.Context) ([]Device, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()
	return m.devices, m.err
}

// CallCount returns how many times List was called.
func (m *mockDiscoverer) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// ResetCallCount resets the call counter.
func (m *mockDiscoverer) ResetCallCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = 0
}

func TestCache_FreshReturnsCached(t *testing.T) {
	mock := &mockDiscoverer{
		devices: []Device{
			{Hostname: "host1", Authorized: true},
		},
	}
	cache := NewCache(mock, 5*time.Minute)

	// First call — should hit the discoverer.
	result, err := cache.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].Hostname != "host1" {
		t.Errorf("expected host1, got %v", result)
	}
	if mock.CallCount() != 1 {
		t.Errorf("discoverer called %d times, want 1", mock.CallCount())
	}

	// Second call — should return cached (no new API call).
	result2, err := cache.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result2) != 1 {
		t.Errorf("expected 1 device, got %d", len(result2))
	}
	if mock.CallCount() != 1 {
		t.Errorf("discoverer called %d times after cache hit, want 1", mock.CallCount())
	}

	// Verify the returned slice is a copy, not the same backing array.
	result2[0].Hostname = "modified"
	result3, err := cache.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result3[0].Hostname != "host1" {
		t.Errorf("cached value should be unchanged, got %q", result3[0].Hostname)
	}
}

func TestCache_StaleFetches(t *testing.T) {
	mock := &mockDiscoverer{
		devices: []Device{
			{Hostname: "old-host"},
		},
	}
	// 1ms TTL — expires almost immediately.
	cache := NewCache(mock, 1*time.Millisecond)

	// First call.
	_, err := cache.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for TTL to expire.
	time.Sleep(10 * time.Millisecond)

	// Second call — should fetch again.
	mock.devices = []Device{{Hostname: "new-host"}}
	_, err = cache.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.CallCount() != 2 {
		t.Errorf("discoverer called %d times, want 2", mock.CallCount())
	}
}

func TestCache_EmptyResult(t *testing.T) {
	mock := &mockDiscoverer{devices: []Device{}}
	cache := NewCache(mock, 5*time.Minute)

	// First call with empty result.
	result, err := cache.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 devices, got %d", len(result))
	}

	// Second call — empty result IS cached (we track "fetched" separately).
	// This avoids hammering the API when a tailnet genuinely has no devices.
	if mock.CallCount() != 1 {
		t.Errorf("discoverer called %d times for cached empty result, want 1", mock.CallCount())
	}
}

func TestCache_ErrorNotCached(t *testing.T) {
	mock := &mockDiscoverer{err: context.DeadlineExceeded}
	cache := NewCache(mock, 5*time.Minute)

	// First call fails.
	_, err := cache.List(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	// Second call — should retry (errors are not cached).
	mock.err = nil
	mock.devices = []Device{{Hostname: "recovered"}}
	_, err = cache.List(context.Background())
	if err != nil {
		t.Fatalf("expected success on retry, got: %v", err)
	}
	if mock.CallCount() != 2 {
		t.Errorf("discoverer called %d times, want 2", mock.CallCount())
	}
}
