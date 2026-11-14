package oslock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func openLockFile(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func TestTryExcludesASecondHolderUntilTheFirstCloses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")
	first, second := openLockFile(t, path), openLockFile(t, path)

	steps := []struct {
		name string
		act  func() error
		want error
	}{
		{"first takes it", func() error { return Try(first) }, nil},
		{"second is refused", func() error { return Try(second) }, ErrHeld},
		{"first closes", first.Close, nil},
		{"second takes it", func() error { return Try(second) }, nil},
	}
	for _, step := range steps {
		if err := step.act(); !errors.Is(err, step.want) {
			t.Fatalf("%s: err = %v, want %v", step.name, err, step.want)
		}
	}
}
