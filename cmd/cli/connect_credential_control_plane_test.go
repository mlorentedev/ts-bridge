package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/config"
	"ts-bridge/internal/profile"
)

const (
	boundControlURL = "https://hs.office.example"
	plantedURL      = "https://evil.example"
	managedKey      = "tskey-auth-profile-secret"
	// keyFileSentinel in a flag list stands for a freshly written explicit key file.
	keyFileSentinel = "<key-file>"
)

// A profile's managed credential must only ever be sent to that profile's
// control plane. connect and browser auto-load ./.env, and that layer (like the
// environment, --control-url and the YAML config) outranks the profile, so a
// planted override would otherwise redirect the managed key silently (#617).
func TestManagedCredentialIsBoundToTheProfileControlPlane(t *testing.T) {
	tests := []struct {
		name        string
		dotenv      string
		env         string
		flags       []string
		yaml        string
		authKey     string
		connectOnly bool   // browser ignores TS_AUTHKEY
		wantLayer   string // empty: the credential is sent
	}{
		{name: "profile alone"},
		{name: "env var names the same URL", env: boundControlURL + "/"},
		{name: "planted .env", dotenv: "TS_CONTROL_URL=" + plantedURL + "\n", wantLayer: "TS_CONTROL_URL"},
		{name: "environment", env: plantedURL, wantLayer: "TS_CONTROL_URL"},
		{name: "flag", flags: []string{"--control-url", plantedURL}, wantLayer: "--control-url"},
		{name: "YAML config", yaml: "control_url: " + plantedURL + "\n", wantLayer: "YAML config"},
		{name: "explicit key file is not the managed credential", env: plantedURL, flags: []string{"--auth-key-file", keyFileSentinel}},
		{name: "explicit key is not the managed credential", env: plantedURL, authKey: "tskey-auth-test-secret", connectOnly: true},
	}
	for _, kind := range []string{"connect", "browser"} {
		for _, tt := range tests {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				if tt.connectOnly && kind != "connect" {
					t.Skip("browser ignores TS_AUTHKEY")
				}
				path := setupProfileRuntimeTest(t)
				bindOfficeCredential(t, path, boundControlURL)
				got, err := runBoundProfile(t, kind, tt.dotenv, tt.env, tt.yaml, tt.authKey, tt.flags)
				if tt.wantLayer != "" {
					assertCredentialRefused(t, err, got, tt.wantLayer)
					return
				}
				if err != nil {
					t.Fatalf("unexpected refusal: %v", err)
				}
				if tt.authKey == "" && len(tt.flags) == 0 && got.AuthKey != managedKey {
					t.Fatalf("managed credential not loaded: AuthKey = %q", got.AuthKey)
				}
			})
		}
	}
}

func assertCredentialRefused(t *testing.T, err error, got config.Config, layer string) {
	t.Helper()
	if err == nil {
		t.Fatalf("credential sent to %q (AuthKey set: %v), want a refusal", got.ControlURL, got.AuthKey != "")
	}
	for _, want := range []string{boundControlURL, plantedURL, layer} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}
	if got.AuthKey != "" {
		t.Errorf("runner reached with AuthKey %q after a refusal", got.AuthKey)
	}
}

// runBoundProfile runs connect or browser for profile "office" with the given
// override layers and returns the config handed to the runner.
func runBoundProfile(t *testing.T, kind, dotenv, env, yaml, authKey string, flags []string) (config.Config, error) {
	t.Helper()
	if dotenv != "" {
		if err := os.WriteFile(".env", []byte(dotenv), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TS_CONTROL_URL", env)
	t.Setenv("TS_AUTHKEY", authKey)
	args := []string{kind, "--profile", "office"}
	if yaml != "" {
		yamlPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(yamlPath, []byte(yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--config", yamlPath)
	}
	if kind == "browser" {
		args = append(args, "--url", "https://forge.example.internal/", "--route", "forge.example.internal:443=mesh:443")
	}
	flags = append([]string(nil), flags...) // the table rows are shared by connect and browser
	for i, flag := range flags {
		if flag == keyFileSentinel {
			keyPath := filepath.Join(t.TempDir(), "key")
			if err := os.WriteFile(keyPath, []byte("tskey-auth-test-secret\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			flags[i] = keyPath
		}
	}
	var got config.Config
	Runner = func(cfg config.Config) error { got = cfg; return nil }
	BrowserRunner = func(cfg config.Config, _ BrowserOptions) error { got = cfg; return nil }
	command := NewRootCmd()
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs(append(args, flags...))
	err := command.Execute()
	return got, err
}

func TestImportRefusesToMoveACredentialedProfile(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "profiles.yaml")
	store := profile.NewStore(storePath)
	if err := store.Set("office", "acemagic-office:45000", boundControlURL); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCredential("office", "office-key"); err != nil {
		t.Fatal(err)
	}
	descriptor := "tsb://acemagic-office:45000?cp=" + plantedURL

	err := runImportDirect("office", descriptor, storePath, false)
	if err == nil || !strings.Contains(err.Error(), "--force") ||
		!strings.Contains(err.Error(), boundControlURL) || !strings.Contains(err.Error(), plantedURL) {
		t.Fatalf("import = %v, want a refusal naming both control URLs and --force", err)
	}
	if got, _ := store.Get("office"); got.ControlURL != boundControlURL || got.Credential != "office-key" {
		t.Fatalf("refused import changed the profile: %+v", got)
	}

	output, _ := captureInitOutput(t, func() {
		if err := runImportDirect("office", descriptor, storePath, true); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "Credential reference") || !strings.Contains(output, "cleared") {
		t.Fatalf("forced import output does not say the credential was cleared: %q", output)
	}
	if got, _ := store.Get("office"); got.ControlURL != plantedURL || got.Credential != "" {
		t.Fatalf("forced import left %+v, want the new control URL and no credential", got)
	}
}

func TestInitProfileRefusesToMoveACredentialedProfileWithoutForce(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "profiles.yaml")
	store := profile.NewStore(storePath)
	if err := store.Set("office", "acemagic-office:45000", boundControlURL); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCredential("office", "office-key"); err != nil {
		t.Fatal(err)
	}
	// Without --force the existing profile is never touched.
	if err := runInitProfile(storePath, initFlags{Profile: "office", ControlURL: plantedURL}); err == nil {
		t.Fatal("init --profile over an existing profile succeeded without --force")
	}
	output, _ := captureInitOutput(t, func() {
		flags := initFlags{Profile: "office", Target: "acemagic-office:45000", ControlURL: plantedURL, Force: true}
		if err := runInitProfile(storePath, flags); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "Credential reference") {
		t.Fatalf("forced init output does not say the credential was cleared: %q", output)
	}
	if got, _ := store.Get("office"); got.ControlURL != plantedURL || got.Credential != "" {
		t.Fatalf("forced init left %+v, want the new control URL and no credential", got)
	}
}
