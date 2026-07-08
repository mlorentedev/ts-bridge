package cmd

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"testing"

	"ts-bridge/internal/config"
)

func TestFormatReadyLine(t *testing.T) {
	got := formatReadyLine("127.0.0.1:16443", "100.64.0.11:6443")
	want := "READY local=127.0.0.1:16443 target=100.64.0.11:6443"
	if got != want {
		t.Errorf("formatReadyLine = %q, want %q", got, want)
	}
}

func TestFormatErrorLine(t *testing.T) {
	got := formatErrorLine(reasonBadAuthKey, `invalid preauth key: hskey-abc`)
	want := `ERROR reason=bad_authkey detail="invalid preauth key: hskey-abc"`
	if got != want {
		t.Errorf("formatErrorLine = %q, want %q", got, want)
	}
}

func TestEscapeSignalDetail(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "dial timeout", "dial timeout"},
		{"quotes", `say "hi"`, `say \"hi\"`},
		{"newline collapses to space", "line1\nline2", "line1 line2"},
		{"crlf collapses", "a\r\nb", "a b"},
		{"combined and trimmed", "  a \"b\"\nc  ", `a \"b\" c`},
		{"trailing backslash (Windows path)", `open C:\tmp\`, `open C:\\tmp\\`},
		{"backslash before quote", `a\"b`, `a\\\"b`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeSignalDetail(tc.in); got != tc.want {
				t.Errorf("escapeSignalDetail(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestEmitReady(t *testing.T) {
	var buf bytes.Buffer
	emitReady(&buf, "127.0.0.1:16443", "t:6443")
	if got := buf.String(); got != "READY local=127.0.0.1:16443 target=t:6443\n" {
		t.Errorf("emitReady wrote %q", got)
	}
}

// TestEmitReady_UsesBoundAddr proves the READY line reflects the actual
// bound listener address (an OS-assigned port), not a nominal ":0" request.
func TestEmitReady_UsesBoundAddr(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	var buf bytes.Buffer
	emitReady(&buf, ln.Addr().String(), "t:6443")

	line := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(line, "READY local=127.0.0.1:") {
		t.Fatalf("READY line %q missing bound-addr prefix", line)
	}
	if strings.Contains(line, "local=127.0.0.1:0 ") {
		t.Errorf("READY line used the nominal :0 request, not the bound port: %q", line)
	}
}

func TestEmitError(t *testing.T) {
	var buf bytes.Buffer
	emitError(&buf, reasonControlPlaneUnreachable, "dial vpn:443: timeout")
	want := "ERROR reason=control_plane_unreachable detail=\"dial vpn:443: timeout\"\n"
	if got := buf.String(); got != want {
		t.Errorf("emitError wrote %q, want %q", got, want)
	}
}

func TestDiagnoseTailscaleInitError_Reason(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantReason string
	}{
		{"nil error", nil, ""},
		{"api key missing", errors.New("api key does not exist"), reasonBadAuthKey},
		{"invalid key", errors.New("invalid key supplied"), reasonBadAuthKey},
		{"key expired", errors.New("auth key expired"), reasonBadAuthKey},
		{"deadline", errors.New("context deadline exceeded"), reasonControlPlaneUnreachable},
		{"io timeout", errors.New("dial tcp: i/o timeout"), reasonControlPlaneUnreachable},
		{"unrecognized", errors.New("something totally different"), reasonUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, _, _ := diagnoseTailscaleInitError(tc.err)
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

func TestWriteStartupBanner_QuietSuppresses(t *testing.T) {
	cfg := config.Config{Hostname: "h", LocalAddr: "127.0.0.1:16443", Target: "t:6443"}

	var quiet bytes.Buffer
	cfg.Quiet = true
	writeStartupBanner(&quiet, cfg)
	if quiet.Len() != 0 {
		t.Errorf("quiet banner wrote %d bytes, want 0: %q", quiet.Len(), quiet.String())
	}

	var loud bytes.Buffer
	cfg.Quiet = false
	writeStartupBanner(&loud, cfg)
	if !strings.Contains(loud.String(), "TAILSCALE BRIDGE") {
		t.Errorf("non-quiet banner missing header: %q", loud.String())
	}
}
