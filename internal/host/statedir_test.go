package host

import (
	"io/fs"
	"strings"
	"testing"
	"time"
)

// fakeInfo is the fs.FileInfo of a state directory candidate.
type fakeInfo struct {
	mode fs.FileMode
}

func (f fakeInfo) Name() string       { return "serve" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return nil }

func TestCheckUnixStateDir(t *testing.T) {
	tests := []struct {
		name    string
		mode    fs.FileMode
		uid     uint32
		wantErr string // "" accepts
	}{
		{"0700 root", fs.ModeDir | 0o700, 0, ""},
		{"group can read", fs.ModeDir | 0o750, 0, "mode 0750"},
		{"others can write", fs.ModeDir | 0o707, 0, "mode 0707"},
		{"owned by a user", fs.ModeDir | 0o700, 1000, "uid 1000"},
		{"a regular file", 0o700, 0, "not a directory"},
		{"a symlink", fs.ModeSymlink | 0o777, 0, "not a directory"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkUnixStateDir("/var/lib/ts-bridge", fakeInfo{tc.mode}, tc.uid)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("checkUnixStateDir() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "/var/lib/ts-bridge") {
				t.Errorf("checkUnixStateDir() = %v, want an error naming the path and %q", err, tc.wantErr)
			}
		})
	}
}
