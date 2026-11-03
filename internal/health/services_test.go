package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func readyBody(t *testing.T, h http.Handler) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return rec.Code, body
}

func TestStartServer_Services(t *testing.T) {
	report := func() (string, []ServiceState) {
		return "lab-01", []ServiceState{{Kind: "rdp", Port: 3390, Published: true}, {Kind: "ssh", Port: 22}}
	}
	wantServices := []any{
		map[string]any{"kind": "rdp", "port": float64(3390), "published": true},
		map[string]any{"kind": "ssh", "port": float64(22), "published": false},
	}
	var status TunnelStatus
	status.MarkReady()
	code, body := readyBody(t, Handler(&status, report))
	if code != http.StatusOK || body["status"] != "ok" || body["node"] != "lab-01" || !reflect.DeepEqual(body["services"], wantServices) {
		t.Errorf("ready: %d %v, want 200 with node and services", code, body)
	}
	status.Store(false, "control_plane_unreachable")
	code, body = readyBody(t, Handler(&status, report))
	if code != http.StatusServiceUnavailable || body["reason"] != "control_plane_unreachable" || body["node"] != "lab-01" || !reflect.DeepEqual(body["services"], wantServices) {
		t.Errorf("not ready: %d %v, want 503 with reason, node and services", code, body)
	}

	// Without a report the body is connect's, unchanged.
	status.MarkReady()
	if _, body := readyBody(t, Handler(&status, nil)); !reflect.DeepEqual(body, map[string]any{"status": "ok"}) {
		t.Errorf("no report: body %v, want only status", body)
	}
	status.Store(false, "x")
	if _, body := readyBody(t, Handler(&status, nil)); !reflect.DeepEqual(body, map[string]any{"status": "not_ready", "reason": "x"}) {
		t.Errorf("no report, not ready: body %v", body)
	}
}
