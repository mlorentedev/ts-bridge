package host

import (
	"io/fs"
	"strings"
	"testing"
)

func TestCheckUnixSecretFile(t *testing.T) {
	tests := []struct {
		name    string
		mode    fs.FileMode
		uid     uint32
		wantErr string // "" accepts
	}{
		{"0600 root", 0o600, 0, ""},
		{"0400 root", 0o400, 0, ""},
		{"group can read", 0o640, 0, "mode 0640"},
		{"others can read", 0o604, 0, "mode 0604"},
		{"owned by a user", 0o600, 1000, "uid 1000"},
		{"a directory", fs.ModeDir | 0o700, 0, "not a regular file"},
		{"a symlink", fs.ModeSymlink | 0o600, 0, "not a regular file"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkUnixSecretFile("/etc/ts-bridge/authkey", fakeInfo{tc.mode}, tc.uid)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("checkUnixSecretFile() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "/etc/ts-bridge/authkey") {
				t.Errorf("checkUnixSecretFile() = %v, want an error naming the path and %q", err, tc.wantErr)
			}
		})
	}
}
