package cmd_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cmdpkg "ts-bridge/cmd/cli"
	"ts-bridge/internal/host"
)

// noRead fails the test if host init reads stdin: it must refuse first.
type noRead struct{ t *testing.T }

func (r noRead) Read([]byte) (int, error) {
	r.t.Error("host init read stdin before refusing")
	return 0, io.EOF
}

// runHostInit runs `host init` as goos with args and stdin.
func runHostInit(t *testing.T, goos string, stdin io.Reader, args ...string) (stdout string, err error) {
	t.Helper()
	c := cmdpkg.HostInitCmdForTest(t, goos)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(io.Discard)
	c.SetIn(stdin)
	c.SetArgs(args)
	err = c.Execute()
	return out.String(), err
}

func TestHostInit_RequiresConfig(t *testing.T) {
	for name, args := range map[string][]string{"interactive": nil, "non-interactive": {"--port", "3390"}} {
		t.Run(name, func(t *testing.T) {
			dir := isolateHostEnv(t)
			_, err := runHostInit(t, "windows", noRead{t}, args...)
			if err == nil || !strings.Contains(err.Error(), "--config") {
				t.Errorf("err = %v, want it to name --config", err)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("host init wrote %v without --config", entries)
			}
		})
	}
}

func TestHostInit_WritesYAML(t *testing.T) {
	tests := []struct {
		name, goos, stdin string
		args              []string
		wantPort          int
		wantRule          string // "" means no firewall key
		wantNoSleep       bool
	}{
		{"windows flags", "windows", "", []string{"--port", "3390", "--firewall-rule", "FlagRule", "--no-sleep"}, 3390, "FlagRule", true},
		{"windows defaults", "windows", "", []string{"--port", "3389"}, 3389, host.DefaultFirewallRule(), false},
		{"windows answers", "windows", "3391\nAnswerRule\nno\n", nil, 3391, "AnswerRule", true},
		{"linux flags", "linux", "", []string{"--port", "3392"}, 3392, "", false},
		{"linux answers", "linux", "3393\n", nil, 3393, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolateHostEnv(t)
			path := filepath.Join(dir, "host.yaml")
			if _, err := runHostInit(t, tc.goos, strings.NewReader(tc.stdin), append([]string{"--config", path}, tc.args...)...); err != nil {
				t.Fatalf("host init: %v", err)
			}
			var warn bytes.Buffer
			file, err := host.LoadFile(path, tc.goos, &warn)
			if err != nil || warn.Len() != 0 {
				t.Fatalf("LoadFile = %v, warnings %q; want a clean file", err, warn.String())
			}
			cfg := host.Resolve(tc.goos, file, host.Flags{})
			if cfg.Port != tc.wantPort {
				t.Errorf("Port = %d, want %d", cfg.Port, tc.wantPort)
			}
			if (file.Host.Firewall != nil) != (tc.wantRule != "") || (file.Host.Power != nil) != (tc.goos == "windows") {
				t.Errorf("firewall %v, power %v; want both on windows only", file.Host.Firewall, file.Host.Power)
			}
			if tc.wantRule != "" && (cfg.FirewallRule != tc.wantRule || cfg.NoSleep != tc.wantNoSleep) {
				t.Errorf("FirewallRule %q, NoSleep %v; want %q, %v", cfg.FirewallRule, cfg.NoSleep, tc.wantRule, tc.wantNoSleep)
			}
			if _, err := os.Stat(filepath.Join(dir, ".env")); !os.IsNotExist(err) {
				t.Errorf("host init created a .env (stat err %v)", err)
			}
			if info, err := os.Stat(path); err == nil && runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
				t.Errorf("mode %04o, want 0644 so an unelevated host check can read it", info.Mode().Perm())
			}
		})
	}
	t.Run("windows-only flags on linux", func(t *testing.T) {
		dir := isolateHostEnv(t)
		_, err := runHostInit(t, "linux", noRead{t}, "--config", filepath.Join(dir, "host.yaml"), "--no-sleep")
		if err == nil || !strings.Contains(err.Error(), "Windows only") {
			t.Errorf("err = %v, want --no-sleep refused on linux", err)
		}
	})
	t.Run("darwin writes nothing", func(t *testing.T) {
		dir := isolateHostEnv(t)
		path := filepath.Join(dir, "host.yaml")
		out, err := runHostInit(t, "darwin", noRead{t}, "--config", path, "--port", "3390")
		if err != nil || !strings.Contains(out, "not applicable") {
			t.Errorf("err %v, stdout %q; want exit 0 and not applicable", err, out)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("darwin wrote %s", path)
		}
		// Like host setup, macOS does not demand a path it will not write.
		if out, err := runHostInit(t, "darwin", noRead{t}); err != nil || !strings.Contains(out, "not applicable") {
			t.Errorf("darwin without --config: err %v, stdout %q; want exit 0 and not applicable", err, out)
		}
	})
	// The documented paths (/etc/ts-bridge, C:\ProgramData\ts-bridge) do not exist
	// on a fresh host (review of #579).
	t.Run("a missing parent is created", func(t *testing.T) {
		dir := isolateHostEnv(t)
		path := filepath.Join(dir, "etc", "ts-bridge", "host.yaml")
		if _, err := runHostInit(t, "linux", noRead{t}, "--config", path, "--port", "3390"); err != nil {
			t.Fatalf("host init: %v", err)
		}
		if _, err := host.LoadFile(path, "linux", nil); err != nil {
			t.Errorf("LoadFile = %v", err)
		}
		if info, err := os.Stat(filepath.Dir(path)); err == nil && runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
			t.Errorf("parent mode %04o, want 0755 so an unelevated host check can traverse it", info.Mode().Perm())
		}
		if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
			t.Errorf("parent holds %v, want only host.yaml (no leftover temp file)", entries)
		}
	})
}

func TestHostInit_Overwrite(t *testing.T) {
	const old = "version: 1\n# hand-written\n"
	tests := []struct {
		name, stdin string
		args        []string
		wantErr     string // "" means overwritten
	}{
		{"non-interactive refuses", "", []string{"--port", "3390"}, "already exists (use --force to overwrite)"},
		{"interactive no aborts", "n\n", nil, "aborted"},
		{"interactive yes overwrites", "y\n3390\nRule\nyes\n", nil, ""},
		{"force overwrites", "", []string{"--port", "3390", "--force"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolateHostEnv(t)
			path := writeTestFile(t, filepath.Join(dir, "host.yaml"), old)
			_, err := runHostInit(t, "windows", strings.NewReader(tc.stdin), append([]string{"--config", path}, tc.args...)...)
			got, _ := os.ReadFile(path)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || string(got) != old {
					t.Errorf("err %v, content %q; want %q and the file untouched", err, got, tc.wantErr)
				}
				return
			}
			if err != nil || !strings.Contains(string(got), "port: 3390") {
				t.Errorf("err %v, content %q; want it overwritten with port 3390", err, got)
			}
		})
	}
	t.Run("a directory is refused even with --force", func(t *testing.T) {
		dir := isolateHostEnv(t)
		path := filepath.Join(dir, "host.yaml")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := runHostInit(t, "windows", noRead{t}, "--config", path, "--port", "3390", "--force"); err == nil || !strings.Contains(err.Error(), "directory") {
			t.Errorf("err = %v, want the directory refused", err)
		}
	})
	t.Run("an unreadable file is refused even with --force", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("permission bits cannot make a file unreadable here")
		}
		dir := isolateHostEnv(t)
		path := writeTestFile(t, filepath.Join(dir, "host.yaml"), old)
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
		_, err := runHostInit(t, "linux", noRead{t}, "--config", path, "--port", "3390", "--force")
		_ = os.Chmod(path, 0o600)
		got, _ := os.ReadFile(path)
		if err == nil || !strings.Contains(err.Error(), "cannot be read") || string(got) != old {
			t.Errorf("err %v, content %q; want it refused and untouched", err, got)
		}
	})
}
