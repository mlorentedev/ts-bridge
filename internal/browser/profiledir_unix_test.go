//go:build !windows

package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareProfileDir(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		wantErr string
	}{
		{name: "a missing directory is created private"},
		{name: "a loose directory is tightened", setup: func(t *testing.T, dir string) {
			if err := os.Mkdir(dir, 0o755); err != nil { // #nosec G301 -- the loose mode is the case under test
				t.Fatal(err)
			}
		}},
		{name: "a symlink is refused", wantErr: "not a directory", setup: func(t *testing.T, dir string) {
			if err := os.Symlink(t.TempDir(), dir); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "a file is refused", wantErr: "not a directory", setup: func(t *testing.T, dir string) {
			if err := os.WriteFile(dir, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "profile")
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			err := PrepareProfileDir(dir)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("PrepareProfileDir = %v, want an error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("PrepareProfileDir: %v", err)
			}
			info, err := os.Lstat(dir)
			if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
				t.Fatalf("profile dir = %v (err %v), want a 0700 directory", info.Mode(), err)
			}
		})
	}
}
