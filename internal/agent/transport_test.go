package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
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

func TestPeerIdentityIsTheCurrentUser(t *testing.T) {
	p := testPaths(t)
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

// A server run by another user is refused before a byte is written; the
// expected owner is faked, as a second account is not available in CI.
func TestClientRefusesAForeignServer(t *testing.T) {
	p := testPaths(t)
	startAgent(t, p)
	self, _ := currentIdentity()
	_, err := dialAs(context.Background(), p, self+"0")
	if !errors.Is(err, ErrForeignPeer) {
		t.Fatalf("dialAs err = %v, want ErrForeignPeer", err)
	}
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

// A client that announces a body and never sends it must not hold a serving
// goroutine forever: ReadHeaderTimeout ends at the headers, so the body read
// needs its own bound.
func TestServerDropsARequestWhoseBodyNeverArrives(t *testing.T) {
	old := readTimeout
	readTimeout = 200 * time.Millisecond
	t.Cleanup(func() { readTimeout = old })
	p := testPaths(t)
	startAgent(t, p)

	conn, err := dial(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("POST /v1/status HTTP/1.1\r\nHost: agent\r\nContent-Length: 1\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadAll(conn); err != nil {
		t.Fatalf("the server kept the connection open: %v", err)
	}
}
