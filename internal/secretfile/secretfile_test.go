package secretfile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFile(t *testing.T) {
	tests := []struct {
		name     string
		existing string // empty means the file does not exist
		mode     os.FileMode
	}{
		{name: "creates the file", mode: 0o600},
		{name: "replaces a 0600 file", existing: "old\n", mode: 0o600},
		{name: "tightens a 0644 file", existing: "old\n", mode: 0o644},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".env")
			if tt.existing != "" {
				if err := os.WriteFile(path, []byte(tt.existing), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, tt.mode); err != nil {
					t.Fatal(err)
				}
			}
			if err := WriteFile(path, []byte("TS_AUTHKEY=k\n")); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "TS_AUTHKEY=k\n" {
				t.Fatalf("content = %q, %v", got, err)
			}
			assertOwnerOnly(t, path)
		})
	}
}

func TestWriteFileKeepsTheOriginalWhenTheRenameFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	renameErr := errors.New("injected rename failure")

	err := WriteFileWith(path, []byte("new\n"), func(string, string) error { return renameErr })
	if !errors.Is(err, renameErr) {
		t.Fatalf("WriteFileWith() error = %v, want the injected failure", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old\n" {
		t.Fatalf("file = %q after a failed rename, want it untouched", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries after a failed rename, want only the original", len(entries))
	}
}

func assertOwnerOnly(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return // owner-only is an ACL there, covered by the Harden test
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %04o, want 0600", perm)
	}
}
