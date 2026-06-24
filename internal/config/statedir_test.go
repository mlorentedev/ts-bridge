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

// stateDirFor must resolve the correct per-OS path. Path assertions are
// filepath-semantics-specific, so each case runs only on its own OS; the
// Linux + Windows CI test jobs cover those two branches between them.
func TestStateDirFor(t *testing.T) {
	cases := []struct {
		name string
		goos string
		env  map[string]string
		want string
	}{
		{
			name: "windows under LOCALAPPDATA",
			goos: osWindows,
			env:  map[string]string{"LOCALAPPDATA": `C:\Users\test\AppData\Local`},
			want: filepath.Join(`C:\Users\test\AppData\Local`, appDirName, stateDirLeaf),
		},
		{
			name: "darwin under Application Support",
			goos: osDarwin,
			env:  map[string]string{"HOME": "/Users/test"},
			want: filepath.Join("/Users/test", "Library", "Application Support", appDirName, stateDirLeaf),
		},
		{
			name: "linux honors XDG_STATE_HOME",
			goos: "linux",
			env:  map[string]string{"XDG_STATE_HOME": "/custom/state", "HOME": "/home/test"},
			want: filepath.Join("/custom/state", appDirName, stateDirLeaf),
		},
		{
			name: "linux falls back to ~/.local/state when XDG unset",
			goos: "linux",
			env:  map[string]string{"HOME": "/home/test"},
			want: filepath.Join("/home/test", ".local", "state", appDirName, stateDirLeaf),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.goos != runtime.GOOS {
				t.Skipf("path assertions are %s-specific (filepath semantics differ per OS)", tc.goos)
			}
			env := func(k string) string { return tc.env[k] }
			if got := stateDirFor(tc.goos, env); got != tc.want {
				t.Errorf("stateDirFor(%q) = %q, want %q", tc.goos, got, tc.want)
			}
		})
	}
}

// Even with HOME / LOCALAPPDATA / XDG_STATE_HOME all unset, the result must
// stay absolute (temp fallback) — never a CWD-relative path (#207).
func TestStateDirForAlwaysAbsolute(t *testing.T) {
	got := stateDirFor(runtime.GOOS, func(string) string { return "" })
	if !filepath.IsAbs(got) {
		t.Fatalf("with all env unset, state dir must still be absolute (no CWD leak), got %q", got)
	}
	if !strings.HasPrefix(got, filepath.Clean(os.TempDir())) {
		t.Errorf("expected temp fallback under %q, got %q", os.TempDir(), got)
	}
}

// The live per-platform default must be absolute, per-user, and never the
// legacy CWD-relative default.
func TestStateDirForPlatformLive(t *testing.T) {
	got := StateDirForPlatform()
	if !filepath.IsAbs(got) {
		t.Fatalf("StateDirForPlatform must be absolute, got %q", got)
	}
	if !strings.Contains(got, appDirName) {
		t.Errorf("expected path under %s, got %q", appDirName, got)
	}
	if got == "./ts-state" || got == "ts-state" {
		t.Errorf("must not be the legacy CWD-relative default, got %q", got)
	}
}

// A hostname containing path separators or ".." must not let the ephemeral
// state dir escape the temp state root.
func TestEphemeralStateDirNoTraversal(t *testing.T) {
	tempRoot := filepath.Clean(os.TempDir())
	appRoot := filepath.Join(tempRoot, appDirName)
	for _, hostname := range []string{"../../evil", `..\..\evil`, "/etc/passwd", "a/b/c", "normal-host", ""} {
		t.Run("hostname="+hostname, func(t *testing.T) {
			got := EphemeralStateDir(hostname)
			if !filepath.IsAbs(got) || !strings.HasPrefix(got, appRoot) {
				t.Fatalf("ephemeral dir must stay under %q, got %q", appRoot, got)
			}
			rel, err := filepath.Rel(appRoot, got)
			if err != nil || strings.Contains(rel, "..") || strings.ContainsAny(rel, `/\`) {
				t.Errorf("ephemeral dir must be a single safe segment under %q, got %q (rel %q)", appRoot, got, rel)
			}
		})
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
