package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/discover"
	"ts-bridge/internal/profile"
)

// discover --profile NAME keeps the chosen host as a named profile, the
// current connection model (ADR-012), and leaves ./.env alone (#646). The test
// drives the real command so the flag-to-persistence wiring is what is checked.
func TestDiscoverSavesTheSelectionAsAProfile(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"profile", []string{"--profile", "office", "--port", "3390"}},
		{"profile takes precedence over --auto", []string{"--auto", "--profile", "office", "--port", "3390"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			storePath := filepath.Join(t.TempDir(), "profiles.yaml")
			old := defaultProfileStorePath
			defaultProfileStorePath = storePath
			t.Cleanup(func() { defaultProfileStorePath = old })

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode([]discover.HeadscaleDevice{{Name: "acemagic", IPs: []string{"100.64.0.7"}, Authorized: true}})
			}))
			t.Cleanup(server.Close)
			t.Setenv("TS_CONTROL_URL", server.URL)
			t.Setenv("TS_HEADSCALE_API_KEY", "api-key")
			t.Setenv("TS_TAILNET", "")
			feedStdin(t, "1\n")

			command := newDiscoverCmd()
			command.SetArgs(tt.args)
			command.SilenceErrors, command.SilenceUsage = true, true
			if err := command.Execute(); err != nil {
				t.Fatalf("discover %v: %v", tt.args, err)
			}

			got, err := profile.NewStore(storePath).Get("office")
			if err != nil {
				t.Fatalf("profile office: %v", err)
			}
			if got.Target != "acemagic:3390" || got.ControlURL != server.URL {
				t.Errorf("profile = %+v, want target acemagic:3390 on %s", got, server.URL)
			}
			if _, err := os.Stat(".env"); !os.IsNotExist(err) {
				t.Errorf(".env exists after discover --profile (stat err %v)", err)
			}
		})
	}
}

// feedStdin replaces os.Stdin with a pipe holding input, which is where
// readSelection scans the host number from.
func feedStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old; _ = r.Close() })
}

func TestDiscoverProfileFlagIsRegistered(t *testing.T) {
	cmd := newDiscoverCmd()
	if cmd.Flags().Lookup("profile") == nil {
		t.Fatal("discover has no --profile flag")
	}
	if !strings.Contains(cmd.Long, "--profile") {
		t.Errorf("discover help does not mention --profile:\n%s", cmd.Long)
	}
}

// --port and --auto change meaning once --profile is set (the port goes into the
// profile, and the profile replaces the .env write), so their help says so.
func TestDiscoverHelpDescribesProfileInteraction(t *testing.T) {
	cmd := newDiscoverCmd()
	for _, name := range []string{"port", "auto"} {
		if usage := cmd.Flags().Lookup(name).Usage; !strings.Contains(usage, "--profile") {
			t.Errorf("--%s usage %q does not mention --profile", name, usage)
		}
	}
	for _, line := range []string{"  --port PORT", "  --auto "} {
		found := false
		for _, text := range strings.Split(cmd.Long, "\n") {
			if !strings.HasPrefix(text, line) {
				continue
			}
			found = true
			if !strings.Contains(text, "profile") {
				t.Errorf("Long help line %q does not mention the profile", text)
			}
		}
		if !found {
			t.Errorf("Long help has no line starting %q", line)
		}
	}
}
