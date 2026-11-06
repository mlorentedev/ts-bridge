package profile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A save that dies before the rename leaves the previous store whole; a reader
// never sees a truncated file (#391, the CFG-005 follow-up).
func TestSaveFailureKeepsThePreviousStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	store := NewStore(path)
	if err := store.Set("office", "host:22", ""); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	store.rename = func(string, string) error { return errors.New("disk gone") }
	if err := store.Set("lost", "host:23", ""); err == nil {
		t.Fatal("Set() succeeded although the replace failed")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("store changed by a failed save: %q (err %v), want %q", after, err, before)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Fatalf("failed save left %s behind", entry.Name())
		}
	}
}

// A temporary file abandoned by a killed writer is inert: reads and writes go
// on, and the store is never parsed from it.
func TestAbandonedTemporaryFileDoesNotAffectTheStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	store := NewStore(path)
	if err := store.Set("office", "host:22", ""); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(filepath.Dir(path), ".profiles.yaml-12345.tmp")
	if err := os.WriteFile(stray, []byte("profiles:\n  half: {target: "), 0o600); err != nil {
		t.Fatal(err)
	}
	if names, err := store.List(); err != nil || len(names) != 1 {
		t.Fatalf("List() beside an abandoned temporary file = %v, %v", names, err)
	}
	if err := store.Set("second", "host:23", ""); err != nil {
		t.Fatalf("Set() beside an abandoned temporary file: %v", err)
	}
}

// Windows refuses to replace a file a reader has open; the save retries.
func TestRetryingRenameOutlastsATransientFailure(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, "from"), filepath.Join(dir, "to")
	if err := os.WriteFile(from, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	failures := 2
	rename := retrying(func(from, to string) error {
		if failures > 0 {
			failures--
			return errors.New("sharing violation")
		}
		return os.Rename(from, to)
	})
	if err := rename(from, to); err != nil {
		t.Fatalf("rename after two transient failures = %v", err)
	}
	if got, _ := os.ReadFile(to); string(got) != "new" {
		t.Fatalf("destination = %q, want the replacement", got)
	}

	persistent := errors.New("access denied")
	if err := retrying(func(string, string) error { return persistent })(from, to); !errors.Is(err, persistent) {
		t.Fatalf("persistent failure = %v, want it returned", err)
	}
}
