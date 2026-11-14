//go:build !windows

package agent

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode()
}

func TestPrepareDirModes(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		wantErr string
	}{
		{"absent is created private", func(*testing.T, string) {}, ""},
		{"loose permissions on our own directory are tightened", func(t *testing.T, dir string) {
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}, ""},
		{"a symlink is refused", func(t *testing.T, dir string) {
			target := t.TempDir()
			if err := os.Symlink(target, dir); err != nil {
				t.Fatal(err)
			}
		}, "not a directory"},
		{"a regular file is refused", func(t *testing.T, dir string) {
			if err := os.WriteFile(dir, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}, "not a directory"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(shortDir(t), "state")
			tc.setup(t, dir)
			err := prepareDir(dir)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("prepareDir err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := mode(t, dir).Perm(); got != 0o700 {
				t.Fatalf("directory mode = %o, want 700", got)
			}
		})
	}
}

func TestVerifyDirRefusesAForeignOwner(t *testing.T) {
	dir := t.TempDir()
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyDir(dir, info, os.Geteuid()); err != nil {
		t.Fatalf("own directory: %v", err)
	}
	err = verifyDir(dir, info, os.Geteuid()+1)
	if err == nil || !strings.Contains(err.Error(), "owned by") {
		t.Fatalf("foreign directory err = %v, want an ownership refusal", err)
	}
}

func TestSocketModeIsOwnerOnly(t *testing.T) {
	dir := shortDir(t)
	a := startAgent(t, dir)

	if got := mode(t, a.Endpoint()).Perm(); got != 0o600 {
		t.Fatalf("socket mode = %o, want 600", got)
	}
	if got := mode(t, dir).Perm(); got != 0o700 {
		t.Fatalf("state directory mode = %o, want 700", got)
	}
}

func TestStartReplacesAStaleSocket(t *testing.T) {
	dir := shortDir(t)
	sock := filepath.Join(dir, socketName)
	stale, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false) // what a crashed agent leaves behind
	_ = stale.Close()
	if mode(t, sock)&os.ModeSocket == 0 {
		t.Fatal("setup: no stale socket left")
	}

	startAgent(t, dir)
	if resp := getStatus(t, dir); resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestStartNeverRemovesANonSocket(t *testing.T) {
	dir := shortDir(t)
	sock := filepath.Join(dir, socketName)
	if err := os.WriteFile(sock, []byte("precious"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Start(Options{Dir: dir}); err == nil {
		t.Fatal("Start replaced a regular file")
	}
	if data, err := os.ReadFile(sock); err != nil || string(data) != "precious" {
		t.Fatalf("the file at the socket path was touched: %q, %v", data, err)
	}
}

func TestStartRefusesALongSocketPath(t *testing.T) {
	dir := filepath.Join(shortDir(t), strings.Repeat("d", 120))
	_, err := Start(Options{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("Start err = %v, want a too-long socket path refusal", err)
	}
}
