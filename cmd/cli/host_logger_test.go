package cmd_test

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	cmdpkg "ts-bridge/cmd/cli"
	"ts-bridge/internal/host"
)

// captureStdoutStderr runs fn with os.Stdout and os.Stderr redirected to pipes
// and returns whatever each stream received.
//
// Both streams are swapped before fn runs because slog binds its writer at
// handler construction: a logger built before the swap would keep writing to
// the real stream and the test would silently pass.
//
// The payloads here are a few hundred bytes at most, well under the 64 KiB pipe
// buffer, so writing before reading cannot deadlock.
func captureStdoutStderr(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stdout, os.Stderr = wOut, wErr

	defer func() {
		os.Stdout, os.Stderr = oldStdout, oldStderr
	}()

	fn()

	wOut.Close()
	wErr.Close()

	var outBuf, errBuf strings.Builder
	if _, err := io.Copy(&outBuf, rOut); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if _, err := io.Copy(&errBuf, rErr); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return outBuf.String(), errBuf.String()
}

// TestHostInitLogger_WritesToStderrNotStdout is the regression guard for #254:
// host command logs must go to stderr so stdout carries only command output.
func TestHostInitLogger_WritesToStderrNotStdout(t *testing.T) {
	tests := []struct {
		name      string
		logFormat string
	}{
		{name: "text_format", logFormat: "text"},
		{name: "json_format", logFormat: "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := captureStdoutStderr(t, func() {
				logger := cmdpkg.HostInitLoggerForTest(host.Config{LogFormat: tt.logFormat})
				logger.Info("host check", "rdp_port", 3389)
			})

			if stdout != "" {
				t.Errorf("logger wrote to stdout, want stderr only; stdout = %q", stdout)
			}
			if !strings.Contains(stderr, "host check") {
				t.Errorf("log line missing from stderr; stderr = %q", stderr)
			}
		})
	}
}

// TestPrintCheckJSON_StdoutIsPureJSON asserts the acceptance criterion of #254:
// with the logger active, `host check --json` stdout is a single JSON object.
//
// This is the Go-level equivalent of `ts-bridge host check --json | jq .` and
// the reason the QA-010 smoke test can now assert a real parse.
func TestPrintCheckJSON_StdoutIsPureJSON(t *testing.T) {
	result := host.CheckResult{
		TailscaleIP: "100.64.0.1",
		RDPPort:     3389,
		RDPEnabled:  true,
		FirewallOK:  true,
		TailscaleUp: true,
	}

	stdout, stderr := captureStdoutStderr(t, func() {
		// Mirrors runHostCheck: build the logger, emit a line, then print JSON.
		logger := cmdpkg.HostInitLoggerForTest(host.Config{LogFormat: "text"})
		logger.Info("host check", "tailscale_up", true, "rdp_port", 3389)

		if err := cmdpkg.PrintCheckJSONForTest(result); err != nil {
			t.Errorf("printCheckJSON: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not a single JSON object: %v\n--- stdout ---\n%s", err, stdout)
	}

	if got := parsed["tailscale_ip"]; got != "100.64.0.1" {
		t.Errorf("tailscale_ip = %v, want 100.64.0.1", got)
	}
	if got := parsed["rdp_port"]; got != float64(3389) {
		t.Errorf("rdp_port = %v, want 3389", got)
	}

	// The log line must still be emitted — routed, not silenced.
	if !strings.Contains(stderr, "host check") {
		t.Errorf("log line should survive on stderr; stderr = %q", stderr)
	}
}

// TestPrintSetupJSON_StdoutIsPureJSON is TestPrintCheckJSON_StdoutIsPureJSON for
// the setup path, which #254 reports as sharing the same defect.
func TestPrintSetupJSON_StdoutIsPureJSON(t *testing.T) {
	result := host.SetupResult{
		RDPPort: 3389,
		Steps: []host.SetupStep{
			{Name: "Enable RDP", Success: true, Message: "OK"},
			{Name: "Firewall", Success: false, Message: "skipped"},
		},
	}

	stdout, _ := captureStdoutStderr(t, func() {
		logger := cmdpkg.HostInitLoggerForTest(host.Config{LogFormat: "text"})
		logger.Info("starting host setup", "rdp_port", 3389)

		if err := cmdpkg.PrintSetupJSONForTest(result); err != nil {
			t.Errorf("printSetupJSON: %v", err)
		}
	})

	var parsed cmdpkg.SetupJSONOutputForTest
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not a single JSON object: %v\n--- stdout ---\n%s", err, stdout)
	}

	if len(parsed.Steps) != 2 {
		t.Errorf("steps = %d, want 2", len(parsed.Steps))
	}
	if len(parsed.Warnings) != 1 {
		t.Errorf("warnings = %d, want 1 (the failed step)", len(parsed.Warnings))
	}
}

// TestNewHostLogger_HonorsFormatAndVerbose covers the logger's config wiring
// against an injected writer — no os.Stderr swap needed.
func TestNewHostLogger_HonorsFormatAndVerbose(t *testing.T) {
	tests := []struct {
		name       string
		cfg        host.Config
		wantJSON   bool
		wantDebug  bool
		debugInMsg string
	}{
		{
			name:      "text_info_level",
			cfg:       host.Config{LogFormat: "text"},
			wantJSON:  false,
			wantDebug: false,
		},
		{
			name:      "json_info_level",
			cfg:       host.Config{LogFormat: "json"},
			wantJSON:  true,
			wantDebug: false,
		},
		{
			name:      "verbose_enables_debug",
			cfg:       host.Config{LogFormat: "text", Verbose: true},
			wantJSON:  false,
			wantDebug: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			logger := cmdpkg.NewHostLoggerForTest(tt.cfg, &buf)
			logger.Debug("debug line")
			logger.Info("info line")

			out := buf.String()

			if !strings.Contains(out, "info line") {
				t.Errorf("info line missing; out = %q", out)
			}
			if gotDebug := strings.Contains(out, "debug line"); gotDebug != tt.wantDebug {
				t.Errorf("debug present = %v, want %v; out = %q", gotDebug, tt.wantDebug, out)
			}

			// A JSON handler emits objects; the text handler emits key=value.
			gotJSON := strings.Contains(out, `"msg":"info line"`)
			if gotJSON != tt.wantJSON {
				t.Errorf("JSON format = %v, want %v; out = %q", gotJSON, tt.wantJSON, out)
			}
		})
	}
}
