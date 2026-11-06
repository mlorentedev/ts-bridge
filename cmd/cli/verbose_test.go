package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/config"
)

// The console logger's level comes from cfg.Verbose, so the effect to assert is
// whether a debug record reaches the console of the production logger, not that
// the flag parses. logger.Enabled cannot answer that: the combined logger also
// writes a debug-level file, so it reports debug as enabled either way (#656).
func TestConnectVerboseReachesTheLogger(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want bool
	}{
		{"default stays quiet", "", nil, false},
		{"--verbose", "", []string{"--verbose"}, true},
		{"-v", "", []string{"-v"}, true},
		{"TS_VERBOSE", "1", nil, true},
		{"--verbose=false over TS_VERBOSE", "1", []string{"--verbose=false"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := setupProfileRuntimeTest(t)
			writeProfileRuntimeFixture(t, path)
			logRoot := t.TempDir()
			t.Setenv("HOME", logRoot)
			t.Setenv("LOCALAPPDATA", logRoot)
			t.Setenv("TS_VERBOSE", tt.env)
			restoreConnectLogging(t)
			LoggerInit = InitLogger
			var gotCfg config.Config
			Runner = func(cfg config.Config) error {
				gotCfg = cfg
				logger.Debug("verbose-probe")
				return nil
			}

			stdout, _ := captureInitOutput(t, func() {
				command := NewRootCmd()
				command.SetArgs(append([]string{"connect", "--profile", "office", "--quiet"}, tt.args...))
				if err := command.Execute(); err != nil {
					t.Fatal(err)
				}
			})

			onConsole := strings.Contains(stdout, "verbose-probe")
			if gotCfg.Verbose != tt.want || onConsole != tt.want {
				t.Errorf("cfg.Verbose = %v, debug record on console = %v, want both %v", gotCfg.Verbose, onConsole, tt.want)
			}
		})
	}
}

func TestBrowserVerboseReachesTheConfig(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want bool
	}{
		{"default stays quiet", "", nil, false},
		{"--verbose", "", []string{"--verbose"}, true},
		{"TS_VERBOSE", "1", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// browser runs the real logger init: keep its log file out of the real profile.
			logRoot := t.TempDir()
			t.Setenv("HOME", logRoot)
			t.Setenv("LOCALAPPDATA", logRoot)
			t.Setenv("TS_VERBOSE", tt.env)
			t.Setenv("TS_TARGET", "")
			t.Setenv("TS_AUTHKEY", "")
			t.Setenv("TS_CONTROL_URL", "")
			t.Chdir(t.TempDir())
			keyFile := filepath.Join(t.TempDir(), "authkey")
			if err := os.WriteFile(keyFile, []byte("tskey-auth-test123-secret"), 0600); err != nil {
				t.Fatal(err)
			}
			var got config.Config
			BrowserRunner = func(cfg config.Config, _ BrowserOptions) error { got = cfg; return nil }
			t.Cleanup(func() { BrowserRunner = nil })

			command := NewRootCmd()
			command.SetArgs(append([]string{
				"browser",
				"--url", "https://forge.example.internal/",
				"--route", "forge.example.internal:443=apps:443",
				"--auth-key-file", keyFile,
				"--user-data-dir", filepath.Join(t.TempDir(), "edge"),
			}, tt.args...))
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}

			if got.Verbose != tt.want {
				t.Errorf("cfg.Verbose = %v, want %v", got.Verbose, tt.want)
			}
		})
	}
}
