package health

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"
)

func startTestServer(t *testing.T, status *TunnelStatus) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ready, reason := status.Load()
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			body := map[string]string{"status": "not_ready"}
			if reason != "" {
				body["reason"] = reason
			}
			_ = json.NewEncoder(w).Encode(body)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"active_connections":   0,
			"total_connections":    0,
			"total_bytes_tx":       0,
			"total_bytes_rx":       0,
			"total_errors":         0,
			"rejected_connections": 0,
		})
	})

	return httptest.NewServer(mux)
}

func TestLiveEndpoint_Returns200(t *testing.T) {
	var status TunnelStatus
	srv := startTestServer(t, &status)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health/live")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
}

func TestLiveEndpoint_ContentType(t *testing.T) {
	var status TunnelStatus
	srv := startTestServer(t, &status)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health/live")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestReadyEndpoint_Returns503WhenNotReady(t *testing.T) {
	var status TunnelStatus
	status.Store(false, "")
	srv := startTestServer(t, &status)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if result["status"] != "not_ready" {
		t.Errorf("expected status=not_ready, got %q", result["status"])
	}
}

func TestReadyEndpoint_Returns503WithReasonWhenTerminal(t *testing.T) {
	var status TunnelStatus
	status.Store(false, "tsnet: backend in state Stopped")
	srv := startTestServer(t, &status)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if result["status"] != "not_ready" {
		t.Errorf("expected status=not_ready, got %q", result["status"])
	}
	if result["reason"] != "tsnet: backend in state Stopped" {
		t.Errorf("expected reason in 503 body, got %q", result["reason"])
	}
}

func TestReadyEndpoint_Returns200WhenReady(t *testing.T) {
	var status TunnelStatus
	status.MarkReady()
	srv := startTestServer(t, &status)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
}

func TestReadyEndpoint_TransitionsFromNotReadyToReady(t *testing.T) {
	var status TunnelStatus
	status.Store(false, "")
	srv := startTestServer(t, &status)
	defer srv.Close()

	// First check: not ready
	resp, err := http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 before ready, got %d", resp.StatusCode)
	}

	// Transition to ready
	status.MarkReady()

	// Second check: ready
	resp, err = http.Get(srv.URL + "/health/ready")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 after ready, got %d", resp.StatusCode)
	}
}

func TestMetricsEndpoint_ReturnsJSON(t *testing.T) {
	var status TunnelStatus
	srv := startTestServer(t, &status)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}

	expectedKeys := []string{
		"active_connections",
		"total_connections",
		"total_bytes_tx",
		"total_bytes_rx",
		"total_errors",
		"rejected_connections",
	}
	for _, k := range expectedKeys {
		if _, ok := result[k]; !ok {
			t.Errorf("expected key %q in metrics response", k)
		}
	}
}

func TestMetricsEndpoint_ContentType(t *testing.T) {
	var status TunnelStatus
	srv := startTestServer(t, &status)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

// TestStartServer verifies the real StartServer function creates a working server.
func TestStartServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var status TunnelStatus
	status.MarkReady()

	// Find a free port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Start on the resolved port.
	server := StartServer(addr, &status, logger)
	defer func() {
		_ = server.Close()
	}()

	// StartServer runs ListenAndServe in a goroutine, so the listener may not
	// be accepting the instant it returns — especially on Windows, where
	// rebinding a just-closed port can lag. Poll until ready instead of relying
	// on a fixed sleep, which was flaky across platforms.
	var resp *http.Response
	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err = http.Get("http://" + addr + "/health/live")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("real StartServer request failed after retries: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from real StartServer, got %d", resp.StatusCode)
	}
}
