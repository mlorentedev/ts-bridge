package discover

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListHeadscaleDevices_Success(t *testing.T) {
	// Mock Headscale API server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		response := []HeadscaleDevice{
			{
				ID:         "hs-dev-1",
				Name:       "linux-host",
				User:       "alice",
				IPs:        []string{"100.64.0.1"},
				Authorized: true,
			},
			{
				ID:         "hs-dev-2",
				Name:       "windows-host",
				User:       "bob",
				IPs:        []string{"100.64.0.2"},
				Authorized: true,
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	devices, err := ListHeadscaleDevices(context.Background(), "test-key", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}

	if devices[0].Name != "linux-host" {
		t.Errorf("device[0].Name = %q, want %q", devices[0].Name, "linux-host")
	}
	if devices[0].User != "alice" {
		t.Errorf("device[0].User = %q, want %q", devices[0].User, "alice")
	}
	if devices[1].Name != "windows-host" {
		t.Errorf("device[1].Name = %q, want %q", devices[1].Name, "windows-host")
	}
}

func TestListHeadscaleDevices_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := ListHeadscaleDevices(context.Background(), "bad-key", server.URL)
	if err == nil {
		t.Fatal("expected error for invalid auth key")
	}
	errMsg := err.Error()
	if !contains(errMsg, "auth failed") {
		t.Errorf("error should mention auth failure, got: %s", errMsg)
	}
}

func TestListHeadscaleDevices_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]HeadscaleDevice{})
	}))
	defer server.Close()

	devices, err := ListHeadscaleDevices(context.Background(), "test-key", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
}

func TestListHeadscaleDevices_MissingAPIKey(t *testing.T) {
	_, err := ListHeadscaleDevices(context.Background(), "", "http://localhost:8080")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !contains(err.Error(), "required") {
		t.Errorf("error should mention required, got: %s", err.Error())
	}
}

func TestListHeadscaleDevices_MissingControlURL(t *testing.T) {
	_, err := ListHeadscaleDevices(context.Background(), "test-key", "")
	if err == nil {
		t.Fatal("expected error for missing control URL")
	}
	if !contains(err.Error(), "required") {
		t.Errorf("error should mention required, got: %s", err.Error())
	}
}

func TestListHeadscaleDevices_ObjectWrapper(t *testing.T) {
	// Some Headscale versions return {"devices": [...]} instead of [...].
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string][]HeadscaleDevice{
			"devices": {
				{Name: "wrapped-host", User: "charlie", IPs: []string{"100.64.0.3"}},
			},
		})
	}))
	defer server.Close()

	devices, err := ListHeadscaleDevices(context.Background(), "test-key", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Name != "wrapped-host" {
		t.Errorf("device[0].Name = %q, want %q", devices[0].Name, "wrapped-host")
	}
}

func TestListHeadscaleDevices_Unreachable(t *testing.T) {
	_, err := ListHeadscaleDevices(context.Background(), "test-key", "http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
	if !contains(err.Error(), "request failed") && !contains(err.Error(), "unreachable") {
		t.Logf("error: %s", err.Error())
	}
}

func TestListHeadscaleDevices_StringIP(t *testing.T) {
	// Headscale may return ip as a string instead of array.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Raw JSON with ip as string.
		_, _ = w.Write([]byte(`[{"id":"s","name":"str-ip","user":"u","ip":"100.64.0.5","authorized":true}]`))
	}))
	defer server.Close()

	devices, err := ListHeadscaleDevices(context.Background(), "test-key", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if len(devices[0].IPs) != 1 || devices[0].IPs[0] != "100.64.0.5" {
		t.Errorf("IPs = %v, want [100.64.0.5]", devices[0].IPs)
	}
}

// contains is a case-insensitive substring check.
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
