//go:build !windows

package agent

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testPaths keeps the lock and the socket in separate directories under one
// short root, as the defaults do on Linux.
func testPaths(t *testing.T) Paths {
	t.Helper()
	root := shortDir(t)
	return Paths{Base: filepath.Join(root, "b"), Run: filepath.Join(root, "r")}
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode()
}

func TestStartReplacesAStaleSocket(t *testing.T) {
	p := testPaths(t)
	if err := prepareDir(p.Run); err != nil {
		t.Fatal(err)
	}
	stale, err := net.Listen("unix", filepath.Join(p.Run, socketName))
	if err != nil {
		t.Fatal(err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false) // what a crashed agent leaves behind
	_ = stale.Close()

	startAgent(t, p)
	if resp := mustGet(t, p, "/v1/status"); resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestStartNeverRemovesANonSocket(t *testing.T) {
	p := testPaths(t)
	if err := prepareDir(p.Run); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(p.Run, socketName)
	if err := os.WriteFile(sock, []byte("precious"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(Options{Paths: p}); err == nil {
		t.Fatal("Start replaced a regular file")
	}
	if data, err := os.ReadFile(sock); err != nil || string(data) != "precious" {
		t.Fatalf("the file at the socket path was touched: %q, %v", data, err)
	}
	// The failed start released the lock.
	a, err := Start(Options{Paths: Paths{Base: p.Base, Run: shortDir(t)}})
	if err != nil {
		t.Fatalf("Start after a failed start: %v", err)
	}
	_ = a.Close()
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
		}, ""},
		{"a symlink is refused", func(t *testing.T, dir string) {
			if err := os.Symlink(t.TempDir(), dir); err != nil {
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
	p := testPaths(t)
	a := startAgent(t, p)

	if got := mode(t, a.Endpoint()).Perm(); got != 0o600 {
		t.Fatalf("socket mode = %o, want 600", got)
	}
	for _, dir := range []string{p.Base, p.Run} {
		if got := mode(t, dir).Perm(); got != 0o700 {
			t.Fatalf("%s mode = %o, want 700", dir, got)
		}
	}
}

func TestStartRefusesASocketPathAtTheLimit(t *testing.T) {
	p := testPaths(t)
	// Pad Run so that Run/agent.sock is exactly sunPathMax bytes: one too many
	// for the terminating NUL.
	pad := sunPathMax - len(filepath.Join(p.Run, "x", socketName)) + 1
	p.Run = filepath.Join(p.Run, strings.Repeat("d", pad))
	if n := len(filepath.Join(p.Run, socketName)); n != sunPathMax {
		t.Fatalf("setup: socket path is %d bytes, want %d", n, sunPathMax)
	}
	_, err := Start(Options{Paths: p})
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("Start err = %v, want a too-long socket path refusal", err)
	}
}

func TestClientRefusesAnOpenSocketDirectory(t *testing.T) {
	t.Run("socket directory not private", func(t *testing.T) {
		p := testPaths(t)
		startAgent(t, p)
		if err := os.Chmod(p.Run, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := get(t, p, "/v1/status"); err == nil || !strings.Contains(err.Error(), "mode") {
			t.Fatalf("GET err = %v, want a refusal naming the directory mode", err)
		}
	})
}
