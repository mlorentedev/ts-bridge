//go:build !windows

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startAgent starts an agent on p and serves it until the test ends.
func startAgent(t *testing.T, p Paths) *Agent {
	t.Helper()
	a, err := Start(Options{Paths: p, Version: "test"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Serve: %v", err)
		}
		_ = a.Close()
	})
	return a
}

func get(t *testing.T, p Paths, path string) (*http.Response, error) {
	t.Helper()
	client := HTTPClient(p)
	client.Timeout = 5 * time.Second
	resp, err := client.Get("http://agent" + path)
	if err == nil {
		t.Cleanup(func() { _ = resp.Body.Close() })
	}
	return resp, err
}

func mustGet(t *testing.T, p Paths, path string) *http.Response {
	t.Helper()
	resp, err := get(t, p, path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode()
}

func TestStartRefusesASecondAgent(t *testing.T) {
	p := testPaths(t)
	startAgent(t, p)

	_, err := Start(Options{Paths: p})
	if !errors.Is(err, ErrAlreadyRunning) || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("second Start err = %v, want ErrAlreadyRunning", err)
	}
	// The refused starter must not have removed the first agent's socket.
	if resp := mustGet(t, p, "/v1/status"); resp.StatusCode != http.StatusOK {
		t.Fatalf("status after a refused second start = %d, want 200", resp.StatusCode)
	}
}

func TestStartAfterCloseSucceeds(t *testing.T) {
	p := testPaths(t)
	for i := range 2 {
		a, err := Start(Options{Paths: p})
		if err != nil {
			t.Fatalf("Start %d: %v", i, err)
		}
		if err := a.Close(); err != nil {
			t.Fatalf("Close %d: %v", i, err)
		}
	}
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

func TestPeerIdentityIsTheCurrentUser(t *testing.T) {
	p := testPaths(t)
	if err := prepareDir(p.Run); err != nil {
		t.Fatal(err)
	}
	ln, _, err := listen(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	got := make(chan Identity, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			got <- ""
			return
		}
		defer func() { _ = conn.Close() }()
		id, _ := peerIdentity(conn)
		got <- id
	}()
	conn, err := dial(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	self, _ := currentIdentity()
	select {
	case id := <-got:
		if id == "" || id != self {
			t.Fatalf("peer identity = %q, want the current user %q", id, self)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no connection accepted")
	}
}

func TestClientRefusesAnUntrustedEndpoint(t *testing.T) {
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
	t.Run("server is not this user", func(t *testing.T) {
		p := testPaths(t)
		startAgent(t, p)
		self, _ := currentIdentity()
		_, err := dialAs(context.Background(), p, self+"0")
		if !errors.Is(err, ErrForeignPeer) {
			t.Fatalf("dialAs err = %v, want ErrForeignPeer", err)
		}
	})
}

func TestMetaAndStatusOverTheRealTransport(t *testing.T) {
	p := testPaths(t)
	startAgent(t, p)

	var meta struct {
		AgentVersion string `json:"agent_version"`
		APIMin       int    `json:"api_min"`
		APIMax       int    `json:"api_max"`
	}
	decode(t, mustGet(t, p, "/v1/meta"), http.StatusOK, &meta)
	if meta.AgentVersion != "test" || meta.APIMin != 1 || meta.APIMax != 1 {
		t.Fatalf("meta = %+v, want version test and API 1..1", meta)
	}

	var status struct {
		PID     int    `json:"pid"`
		Started string `json:"started"`
	}
	decode(t, mustGet(t, p, "/v1/status"), http.StatusOK, &status)
	if status.PID != os.Getpid() {
		t.Fatalf("pid = %d, want %d", status.PID, os.Getpid())
	}
	if _, err := time.Parse(time.RFC3339, status.Started); err != nil {
		t.Fatalf("started %q is not RFC 3339: %v", status.Started, err)
	}
}

func TestErrorsUseTheEnvelope(t *testing.T) {
	p := testPaths(t)
	startAgent(t, p)
	client := HTTPClient(p)
	client.Timeout = 5 * time.Second

	for _, tc := range []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodPost, "/v1/status", http.StatusMethodNotAllowed, "method_not_allowed"},
		{http.MethodDelete, "/v1/meta", http.StatusMethodNotAllowed, "method_not_allowed"},
		{http.MethodGet, "/v1/nothing", http.StatusNotFound, "not_found"},
		{http.MethodGet, "/status", http.StatusNotFound, "not_found"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, "http://agent"+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			var body struct{ Code, Message, Detail string }
			decode(t, resp, tc.status, &body)
			if body.Code != tc.code || body.Message == "" {
				t.Fatalf("error body = %+v, want code %q and a message", body, tc.code)
			}
		})
	}
}

func decode(t *testing.T, resp *http.Response, status int, v any) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status = %d, want %d", resp.StatusCode, status)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode: %v", err)
	}
}
