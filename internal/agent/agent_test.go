package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// shortDir returns a state directory with a short path: a Unix socket path is
// limited to about 104 bytes, and t.TempDir embeds the test name.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tsb")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// startAgent starts an agent in dir and serves it until the test ends.
func startAgent(t *testing.T, dir string) *Agent {
	t.Helper()
	a, err := Start(Options{Dir: dir})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
		_ = a.Close()
	})
	return a
}

func TestStartRefusesASecondAgent(t *testing.T) {
	dir := shortDir(t)
	startAgent(t, dir)

	_, err := Start(Options{Dir: dir})
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Start err = %v, want ErrAlreadyRunning", err)
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Fatalf("error %q does not say an agent is already running", err)
	}
}

func TestStartAfterCloseSucceeds(t *testing.T) {
	dir := shortDir(t)
	first, err := Start(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	second, err := Start(Options{Dir: dir})
	if err != nil {
		t.Fatalf("Start after Close: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestStartKeepsTheSocketOfARunningAgent(t *testing.T) {
	dir := shortDir(t)
	startAgent(t, dir)

	if _, err := Start(Options{Dir: dir}); err == nil {
		t.Fatal("second Start succeeded")
	}
	// The refused starter must not have removed the first agent's endpoint.
	resp := getStatus(t, dir)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status after a refused second start = %d, want 200", resp.StatusCode)
	}
}

func getStatus(t *testing.T, dir string) *http.Response {
	t.Helper()
	client := HTTPClient(dir)
	client.Timeout = 5 * time.Second
	resp, err := client.Get("http://agent/v1/status")
	if err != nil {
		t.Fatalf("GET /v1/status: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestStatusOverTheRealTransport(t *testing.T) {
	dir := shortDir(t)
	startAgent(t, dir)

	resp := getStatus(t, dir)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		API     string `json:"api"`
		PID     int    `json:"pid"`
		Started string `json:"started"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.API != "v1" || body.PID != os.Getpid() {
		t.Fatalf("body = %+v, want api v1 and pid %d", body, os.Getpid())
	}
	if _, err := time.Parse(time.RFC3339, body.Started); err != nil {
		t.Fatalf("started %q is not RFC 3339: %v", body.Started, err)
	}
}

func TestStatusRoutes(t *testing.T) {
	dir := shortDir(t)
	startAgent(t, dir)
	client := HTTPClient(dir)
	client.Timeout = 5 * time.Second

	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/v1/status", http.StatusOK},
		{http.MethodPost, "/v1/status", http.StatusMethodNotAllowed},
		{http.MethodGet, "/v1/nothing", http.StatusNotFound},
		{http.MethodGet, "/status", http.StatusNotFound},
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
			_ = resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

func TestPeerIdentityIsTheCurrentUser(t *testing.T) {
	dir := shortDir(t)
	if err := prepareDir(dir); err != nil {
		t.Fatal(err)
	}
	ln, err := listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	type accepted struct {
		id  Identity
		err error
	}
	got := make(chan accepted, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			got <- accepted{err: err}
			return
		}
		defer func() { _ = conn.Close() }()
		id, err := peerIdentity(conn)
		got <- accepted{id, err}
	}()
	conn, err := dial(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	select {
	case res := <-got:
		if res.err != nil {
			t.Fatalf("peerIdentity: %v", res.err)
		}
		self, err := currentIdentity()
		if err != nil {
			t.Fatal(err)
		}
		if res.id == "" || res.id != self {
			t.Fatalf("peer identity = %q, want the current user %q", res.id, self)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no connection accepted")
	}
}
