package profile

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

// Writers that load, mutate and save the shared YAML must not lose each other's
// profiles (#391). Each goroutine owns its Store, as separate processes do.
func TestConcurrentWritersLoseNoProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	const writers = 24

	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- NewStore(path).Set(fmt.Sprintf("p%02d", i), "host:22", "")
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Set: %v", err)
		}
	}

	names, err := NewStore(path).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != writers {
		t.Fatalf("store holds %d profiles after %d concurrent writers, want all of them", len(names), writers)
	}
}
