package discover

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestListHeadscaleDevices_Success serves the node list a real Headscale
// v0.29.7 returned (keys scrubbed, nothing else changed) at the
// path Headscale serves it. The old fixture was encoded from HeadscaleDevice
// itself, so it agreed with the decoder whatever Headscale sent (#717).
func TestListHeadscaleDevices_Success(t *testing.T) {
	payload, err := os.ReadFile("testdata/headscale-v0.29.7-nodes.json")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/api/v1/node" {
			http.Error(w, `{"code":5,"message":"Not Found","details":[]}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	devices, err := ListHeadscaleDevices(context.Background(), "test-key", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []HeadscaleDevice{
		{ID: "1", Name: "tsb-e2e-linux", User: "tagged-devices", IPs: []string{"100.64.0.1", "fd7a:115c:a1e0::1"},
			LastSeen: "2026-11-11T07:42:53.725122966Z", Authorized: true, Online: true},
		{ID: "2", Name: "tsb-e2e-client", User: "tagged-devices", IPs: []string{"100.64.0.2", "fd7a:115c:a1e0::2"},
			LastSeen: "2026-11-11T07:43:10.255774237Z", Authorized: true},
	}
	for i := range want {
		devices[i].ipRaw = nil
	}
	if !reflect.DeepEqual(devices, want) {
		t.Errorf("devices:\n got %+v\nwant %+v", devices, want)
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
		_, _ = w.Write([]byte(`{"nodes":[]}`)) // what v0.29.7 answers with no node
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

// TestListHeadscaleDevices_RejectsAnotherShape: a body that is not a node list
// is an error, not an empty tailnet.
func TestListHeadscaleDevices_RejectsAnotherShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"host","ip":"100.64.0.1"}]`))
	}))
	defer server.Close()
	if _, err := ListHeadscaleDevices(context.Background(), "test-key", server.URL); err == nil || !strings.Contains(err.Error(), "parse response") {
		t.Errorf("err = %v, want a parse error", err)
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

func TestListHeadscaleDevices_Unreachable(t *testing.T) {
	_, err := ListHeadscaleDevices(context.Background(), "test-key", "http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
	if !contains(err.Error(), "request failed") && !contains(err.Error(), "unreachable") {
		t.Logf("error: %s", err.Error())
	}
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// A diagnosed failure must still wrap the transport error so callers can match
// it with errors.Is (the remediation is appended, not substituted).
func TestListHeadscaleDevices_DiagnosedErrorWrapsCause(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := ListHeadscaleDevices(ctx, "test-key", "http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected an error from an expired context")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, DeadlineExceeded) = false for %v", err)
	}
}
