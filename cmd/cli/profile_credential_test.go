package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/config"
	"ts-bridge/internal/credential"
	"ts-bridge/internal/profile"
)

func setupProfileCredentialTest(t *testing.T) (string, string) {
	t.Helper()
	profilePath := filepath.Join(t.TempDir(), "profiles.yaml")
	credentialDir := filepath.Join(t.TempDir(), "credentials")
	profiles := profile.NewStore(profilePath)
	if err := profiles.Set("office", "acemagic-office:45000", ""); err != nil {
		t.Fatal(err)
	}
	if err := profiles.SetCredential("office", "office-key"); err != nil {
		t.Fatal(err)
	}
	if err := credential.NewStore(credentialDir).Set("office-key", "tskey-auth-profile-secret", false); err != nil {
		t.Fatal(err)
	}

	oldProfilePath := defaultProfileStorePath
	oldCredentialDir := defaultCredentialStoreDir
	t.Cleanup(func() {
		defaultProfileStorePath = oldProfilePath
		defaultCredentialStoreDir = oldCredentialDir
	})
	defaultProfileStorePath = profilePath
	defaultCredentialStoreDir = credentialDir
	return profilePath, credentialDir
}

func TestConnectResolvesProfileCredential(t *testing.T) {
	setupProfileCredentialTest(t)
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_CONTROL_URL", "")
	oldRunner, oldLogger := Runner, LoggerInit
	t.Cleanup(func() { Runner, LoggerInit = oldRunner, oldLogger })
	LoggerInit = nil

	var captured config.Config
	Runner = func(cfg config.Config) error {
		captured = cfg
		return nil
	}
	command := newConnectCmd()
	command.SetArgs([]string{"--profile", "office"})
	if err := command.Execute(); err != nil {
		t.Fatalf("connect error = %v", err)
	}
	if captured.AuthKey != "tskey-auth-profile-secret" {
		t.Fatal("connect did not resolve the profile credential")
	}
}

func TestConnectExplicitCredentialSourcesOverrideProfile(t *testing.T) {
	setupProfileCredentialTest(t)
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_CONTROL_URL", "")
	oldRunner, oldLogger := Runner, LoggerInit
	t.Cleanup(func() { Runner, LoggerInit = oldRunner, oldLogger })
	LoggerInit = nil

	tests := []struct {
		name    string
		envKey  string
		args    []string
		wantKey string
	}{
		{name: "environment", envKey: "tskey-auth-env-secret", wantKey: "tskey-auth-env-secret"},
		{name: "auth key file", args: []string{"--auth-key-file"}, wantKey: "tskey-auth-file-secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TS_AUTHKEY", tt.envKey)
			args := []string{"--profile", "office"}
			if len(tt.args) > 0 {
				path := filepath.Join(t.TempDir(), "authkey")
				if err := os.WriteFile(path, []byte(tt.wantKey), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, tt.args[0], path)
			}
			var captured config.Config
			Runner = func(cfg config.Config) error {
				captured = cfg
				return nil
			}
			command := newConnectCmd()
			command.SetArgs(args)
			if err := command.Execute(); err != nil {
				t.Fatalf("connect error = %v", err)
			}
			if captured.AuthKey != tt.wantKey {
				t.Fatalf("AuthKey = %q, want explicit source", captured.AuthKey)
			}
		})
	}
}

func TestConnectAuthKeyFileOverridesMissingProfileCredential(t *testing.T) {
	profilePath, credentialDir := setupProfileCredentialTest(t)
	if err := os.Remove(filepath.Join(credentialDir, "office-key.key")); err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "authkey")
	if err := os.WriteFile(keyFile, []byte("tskey-auth-file-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_CONTROL_URL", "")
	oldRunner, oldLogger := Runner, LoggerInit
	t.Cleanup(func() { Runner, LoggerInit = oldRunner, oldLogger })
	LoggerInit = nil

	var captured config.Config
	Runner = func(cfg config.Config) error {
		captured = cfg
		return nil
	}
	command := newConnectCmd()
	command.SetArgs([]string{"--profile", "office", "--auth-key-file", keyFile})
	if err := command.Execute(); err != nil {
		t.Fatalf("connect error = %v (profile store %s)", err, profilePath)
	}
	if captured.AuthKey != "tskey-auth-file-secret" {
		t.Fatalf("AuthKey = %q, want explicit file key", captured.AuthKey)
	}
}

func TestConnectEnvOverridesMissingProfileCredential(t *testing.T) {
	_, credentialDir := setupProfileCredentialTest(t)
	if err := os.Remove(filepath.Join(credentialDir, "office-key.key")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TS_AUTHKEY", "tskey-auth-env-secret")
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_CONTROL_URL", "")
	oldRunner, oldLogger := Runner, LoggerInit
	t.Cleanup(func() { Runner, LoggerInit = oldRunner, oldLogger })
	LoggerInit = nil

	var captured config.Config
	Runner = func(cfg config.Config) error {
		captured = cfg
		return nil
	}
	command := newConnectCmd()
	command.SetArgs([]string{"--profile", "office"})
	if err := command.Execute(); err != nil {
		t.Fatalf("connect error = %v", err)
	}
	if captured.AuthKey != "tskey-auth-env-secret" {
		t.Fatalf("AuthKey = %q, want environment key", captured.AuthKey)
	}
}

func TestBrowserResolvesProfileCredential(t *testing.T) {
	setupProfileCredentialTest(t)
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_CONTROL_URL", "")
	oldRunner, oldLogger := BrowserRunner, LoggerInit
	t.Cleanup(func() { BrowserRunner, LoggerInit = oldRunner, oldLogger })
	LoggerInit = nil

	var captured config.Config
	BrowserRunner = func(cfg config.Config, _ BrowserOptions) error {
		captured = cfg
		return nil
	}
	command := newBrowserCmd()
	command.SetArgs([]string{
		"--profile", "office",
		"--url", "https://forge.example.internal/",
		"--route", "forge.example.internal:443=apps:443",
	})
	if err := command.Execute(); err != nil {
		t.Fatalf("browser error = %v", err)
	}
	if captured.AuthKey != "tskey-auth-profile-secret" {
		t.Fatal("browser did not resolve the profile credential")
	}
}

func TestBrowserManagedProfileCredentialOverridesEnvironmentWithWarning(t *testing.T) {
	setupProfileCredentialTest(t)
	t.Setenv("TS_AUTHKEY", "tskey-auth-env-secret")
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_CONTROL_URL", "")
	oldRunner, oldLogger := BrowserRunner, LoggerInit
	t.Cleanup(func() { BrowserRunner, LoggerInit = oldRunner, oldLogger })
	LoggerInit = nil

	var captured config.Config
	BrowserRunner = func(cfg config.Config, _ BrowserOptions) error {
		captured = cfg
		return nil
	}
	var stderr bytes.Buffer
	command := newBrowserCmd()
	command.SetErr(&stderr)
	command.SetArgs([]string{
		"--profile", "office",
		"--url", "https://forge.example.internal/",
		"--route", "forge.example.internal:443=apps:443",
	})
	if err := command.Execute(); err != nil {
		t.Fatalf("browser error = %v", err)
	}
	if captured.AuthKey != "tskey-auth-profile-secret" {
		t.Fatalf("AuthKey = %q, want managed profile credential", captured.AuthKey)
	}
	if !strings.Contains(stderr.String(), "browser ignores TS_AUTHKEY when --profile supplies a managed credential") {
		t.Fatalf("stderr = %q, want ignored-environment warning", stderr.String())
	}
}

func TestBrowserAuthKeyFileOverridesProfileCredential(t *testing.T) {
	setupProfileCredentialTest(t)
	t.Setenv("TS_AUTHKEY", "tskey-auth-env-secret")
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_CONTROL_URL", "")
	keyFile := filepath.Join(t.TempDir(), "authkey")
	if err := os.WriteFile(keyFile, []byte("tskey-auth-file-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldRunner, oldLogger := BrowserRunner, LoggerInit
	t.Cleanup(func() { BrowserRunner, LoggerInit = oldRunner, oldLogger })
	LoggerInit = nil

	var captured config.Config
	BrowserRunner = func(cfg config.Config, _ BrowserOptions) error {
		captured = cfg
		return nil
	}
	var stderr bytes.Buffer
	command := newBrowserCmd()
	command.SetErr(&stderr)
	command.SetArgs([]string{
		"--profile", "office",
		"--auth-key-file", keyFile,
		"--url", "https://forge.example.internal/",
		"--route", "forge.example.internal:443=apps:443",
	})
	if err := command.Execute(); err != nil {
		t.Fatalf("browser error = %v", err)
	}
	if captured.AuthKey != "tskey-auth-file-secret" {
		t.Fatalf("AuthKey = %q, want file key", captured.AuthKey)
	}
	if strings.Contains(stderr.String(), "browser ignores TS_AUTHKEY") {
		t.Fatalf("stderr = %q, did not expect ignored-environment warning with --auth-key-file", stderr.String())
	}
}

// bindOfficeCredential re-binds profile "office" and its credential to controlURL
// through the store, the way a profile issued for that control plane would be.
func bindOfficeCredential(t *testing.T, storePath, controlURL string) {
	t.Helper()
	store := profile.NewStore(storePath)
	if err := store.ClearCredential("office"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("office", "acemagic-office:45000", controlURL); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCredential("office", "office-key"); err != nil {
		t.Fatal(err)
	}
}

// A profile's managed credential that cannot be read (deleted, or corrupted on
// disk) is fixed by storing it again, so the error names auth set. A malformed
// key from TS_AUTHKEY is not: auth set cannot outrank the env var, so the hint
// would send the user round in a loop.
func TestConnectProfileCredentialLoadErrorPointsToAuthSet(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		damage   func(t *testing.T, keyFile string)
		wantHint bool
	}{
		{name: "deleted", damage: func(t *testing.T, f string) {
			if err := os.Remove(f); err != nil {
				t.Fatal(err)
			}
		}, wantHint: true},
		{name: "corrupted", damage: func(t *testing.T, f string) {
			if err := os.WriteFile(f, []byte("not-a-key\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, wantHint: true},
		{name: "malformed env key", envKey: "not-a-key", damage: func(*testing.T, string) {}, wantHint: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, credentialDir := setupProfileCredentialTest(t)
			tt.damage(t, filepath.Join(credentialDir, "office-key.key"))
			t.Setenv("TS_AUTHKEY", tt.envKey)
			t.Setenv("TS_TARGET", "")
			t.Setenv("TS_CONTROL_URL", "")
			oldRunner, oldLogger := Runner, LoggerInit
			t.Cleanup(func() { Runner, LoggerInit = oldRunner, oldLogger })
			LoggerInit = nil
			Runner = func(config.Config) error { return nil }

			command := newConnectCmd()
			command.SetArgs([]string{"--profile", "office"})
			command.SilenceUsage, command.SilenceErrors = true, true
			err := command.Execute()
			if err == nil {
				t.Fatal("connect succeeded with an unusable credential")
			}
			if got := strings.Contains(err.Error(), "auth set --profile office"); got != tt.wantHint {
				t.Fatalf("error = %q; names auth set = %v, want %v", err, got, tt.wantHint)
			}
			if !tt.wantHint && !strings.Contains(err.Error(), "auth key invalid format") {
				t.Fatalf("error = %q; want the env key's format error, or this case tests another path", err)
			}
		})
	}
}

// browser is a profile command too: a profile with no credential yet must name
// the step that stores one.
func TestBrowserProfileWithoutCredentialPointsToAuthSet(t *testing.T) {
	profilePath, _ := setupProfileCredentialTest(t)
	if err := profile.NewStore(profilePath).Set("bare", "acemagic-office:45000", ""); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_CONTROL_URL", "")
	oldRunner, oldLogger := BrowserRunner, LoggerInit
	t.Cleanup(func() { BrowserRunner, LoggerInit = oldRunner, oldLogger })
	LoggerInit = nil
	BrowserRunner = func(config.Config, BrowserOptions) error { return nil }

	command := newBrowserCmd()
	command.SetArgs([]string{"--profile", "bare", "--url", "https://forge.example.internal/",
		"--route", "forge.example.internal:443=apps:443"})
	command.SilenceUsage, command.SilenceErrors = true, true
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "auth set --profile bare") {
		t.Fatalf("browser --profile bare = %v; want it to name `ts-bridge auth set --profile bare`", err)
	}
}
