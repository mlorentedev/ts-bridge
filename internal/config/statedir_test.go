package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// StateDirForPlatform must always return an absolute, per-user path. A
// CWD-relative result is the #207 bug (node identity leaking into the working
// dir / any git tree).
func TestStateDirForPlatform(t *testing.T) {
	got := StateDirForPlatform()
	if !filepath.IsAbs(got) {
		t.Fatalf("StateDirForPlatform must be absolute, got %q", got)
	}
	if !strings.Contains(got, "ts-bridge") {
		t.Errorf("expected path under ts-bridge, got %q", got)
	}
	if got == "./ts-state" || got == "ts-state" {
		t.Errorf("state dir must not be the legacy CWD-relative default, got %q", got)
	}
}

// Even with HOME / LOCALAPPDATA / XDG_STATE_HOME all unset, the result must
// stay absolute (temp fallback) — never a CWD-relative path.
func TestStateDirForPlatform_EmptyEnvFallsBackToAbsoluteTemp(t *testing.T) {
	empty := func(string) string { return "" }
	got := stateDirFor(runtime.GOOS, empty)
	if !filepath.IsAbs(got) {
		t.Fatalf("with all env unset, state dir must still be absolute (no CWD leak), got %q", got)
	}
	if !strings.HasPrefix(got, filepath.Clean(os.TempDir())) {
		t.Errorf("expected temp fallback under %q, got %q", os.TempDir(), got)
	}
}

// The primary per-user base env var of the current OS must be honored, and the
// app + state leaf appended. Tested on the current OS only so path/filepath
// semantics match (the test matrix runs Linux + Windows).
func TestStateDirForPlatform_HonorsPlatformBaseEnv(t *testing.T) {
	var key, base string
	switch runtime.GOOS {
	case "windows":
		key, base = "LOCALAPPDATA", `C:\Users\test\AppData\Local`
	case "darwin":
		key, base = "HOME", "/Users/test"
	default:
		key, base = "XDG_STATE_HOME", "/home/test/.local/state"
	}
	env := func(k string) string {
		if k == key {
			return base
		}
		return ""
	}
	got := stateDirFor(runtime.GOOS, env)
	if !filepath.IsAbs(got) {
		t.Fatalf("expected absolute path, got %q", got)
	}
	if !strings.HasPrefix(got, filepath.Clean(base)) {
		t.Errorf("expected state dir under %q, got %q", base, got)
	}
	if !strings.Contains(got, "ts-bridge") {
		t.Errorf("expected ts-bridge in path, got %q", got)
	}
}

// A resolved relative state dir must emit a non-blocking leak warning; an
// absolute (or empty) one must not.
func TestWarnRelativeStateDir(t *testing.T) {
	var buf bytes.Buffer
	prev := logger
	SetLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { SetLogger(prev) })

	warnRelativeStateDir("./ts-state")
	if !strings.Contains(buf.String(), "relative") {
		t.Errorf("expected a leak warning for a relative state dir, got %q", buf.String())
	}

	buf.Reset()
	warnRelativeStateDir(StateDirForPlatform())
	if buf.Len() != 0 {
		t.Errorf("expected no warning for an absolute state dir, got %q", buf.String())
	}

	buf.Reset()
	warnRelativeStateDir("")
	if buf.Len() != 0 {
		t.Errorf("expected no warning for an empty state dir, got %q", buf.String())
	}
}

// On Linux, XDG_STATE_HOME must win over the ~/.local/state fallback.
func TestStateDirForPlatform_LinuxXDGOverride(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_STATE_HOME override is Linux-specific")
	}
	env := func(k string) string {
		switch k {
		case "XDG_STATE_HOME":
			return "/custom/state"
		case "HOME":
			return "/home/test"
		default:
			return ""
		}
	}
	got := stateDirFor("linux", env)
	if !strings.HasPrefix(got, "/custom/state") {
		t.Errorf("expected XDG_STATE_HOME to win, got %q", got)
	}
}
