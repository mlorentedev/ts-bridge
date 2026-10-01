package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/config"
	"ts-bridge/internal/credential"
	"ts-bridge/internal/profile"
)

func setupAuthCommandTest(t *testing.T) (string, string) {
	t.Helper()
	profilePath := filepath.Join(t.TempDir(), "profiles.yaml")
	credentialDir := filepath.Join(t.TempDir(), "credentials")

	oldProfilePath := defaultProfileStorePath
	oldCredentialDir := defaultCredentialStoreDir
	oldReader := authSecretReader
	t.Cleanup(func() {
		defaultProfileStorePath = oldProfilePath
		defaultCredentialStoreDir = oldCredentialDir
		authSecretReader = oldReader
	})
	defaultProfileStorePath = profilePath
	defaultCredentialStoreDir = credentialDir
	return profilePath, credentialDir
}

func executeAuthCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := newAuthCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestAuthCommandIsRegistered(t *testing.T) {
	cmd, _, err := NewRootCmd().Find([]string{"auth"})
	if err != nil || cmd == nil || cmd.Name() != "auth" {
		t.Fatalf("auth command not registered: cmd=%v err=%v", cmd, err)
	}
}

func TestAuthSetStoresMaskedCredentialAndBindsProfile(t *testing.T) {
	profilePath, credentialDir := setupAuthCommandTest(t)
	if err := profile.NewStore(profilePath).Set("office", "acemagic-office:45000", ""); err != nil {
		t.Fatal(err)
	}
	const secret = "tskey-auth-super-secret" // #nosec G101 -- synthetic test credential
	authSecretReader = func(string) (string, error) { return secret, nil }

	stdout, stderr, err := executeAuthCommand(t, "set", "office-key", "--profile", "office")
	if err != nil {
		t.Fatalf("auth set error = %v", err)
	}
	if strings.Contains(stdout+stderr, secret) {
		t.Fatal("auth set leaked the credential value")
	}
	if !strings.Contains(stdout, `Credential "office-key" stored for profile "office"`) {
		t.Fatalf("unexpected auth set output: %q", stdout)
	}
	got, err := credential.NewStore(credentialDir).Get("office-key")
	if err != nil || got != secret {
		t.Fatalf("stored credential mismatch: key=%q err=%v", got, err)
	}
	p, err := profile.NewStore(profilePath).Get("office")
	if err != nil || p.Credential != "office-key" {
		t.Fatalf("profile credential = %q, err=%v", p.Credential, err)
	}
}

func TestAuthSetRefusesOverwriteUnlessForced(t *testing.T) {
	profilePath, credentialDir := setupAuthCommandTest(t)
	if err := profile.NewStore(profilePath).Set("office", "host:22", ""); err != nil {
		t.Fatal(err)
	}
	store := credential.NewStore(credentialDir)
	if err := store.Set("office", "tskey-auth-old-secret", false); err != nil {
		t.Fatal(err)
	}
	authSecretReader = func(string) (string, error) { return "tskey-auth-new-secret", nil }

	if _, _, err := executeAuthCommand(t, "set", "--profile", "office"); err == nil {
		t.Fatal("auth set expected overwrite refusal")
	}
	got, _ := store.Get("office")
	if got != "tskey-auth-old-secret" {
		t.Fatal("refused overwrite changed the stored key")
	}
	if _, _, err := executeAuthCommand(t, "set", "--profile", "office", "--force"); err != nil {
		t.Fatalf("forced auth set error = %v", err)
	}
	got, _ = store.Get("office")
	if got != "tskey-auth-new-secret" {
		t.Fatal("forced overwrite did not replace the key")
	}
}

func TestAuthSetReadsExplicitStdinWithoutPrintingCredential(t *testing.T) {
	profilePath, credentialDir := setupAuthCommandTest(t)
	if err := profile.NewStore(profilePath).Set("office", "host:22", ""); err != nil {
		t.Fatal(err)
	}
	const secret = "tskey-auth-from-stdin" // #nosec G101 -- synthetic test credential

	cmd := newAuthCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(secret + "\n"))
	cmd.SetArgs([]string{"set", "--profile", "office", "--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth set --stdin error = %v", err)
	}
	if strings.Contains(stdout.String()+stderr.String(), secret) {
		t.Fatal("auth set --stdin leaked the credential")
	}
	got, err := credential.NewStore(credentialDir).Get("office")
	if err != nil || got != secret {
		t.Fatalf("stdin credential mismatch: key=%q err=%v", got, err)
	}
}

func TestAuthListStatusAndRemoveNeverPrintSecrets(t *testing.T) {
	profilePath, credentialDir := setupAuthCommandTest(t)
	profiles := profile.NewStore(profilePath)
	if err := profiles.Set("office", "host:22", ""); err != nil {
		t.Fatal(err)
	}
	if err := profiles.SetCredential("office", "shared"); err != nil {
		t.Fatal(err)
	}
	const secret = "tskey-auth-never-print" // #nosec G101 -- synthetic test credential
	if err := credential.NewStore(credentialDir).Set("shared", secret, false); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"list"}, {"status", "--profile", "office"}} {
		stdout, stderr, err := executeAuthCommand(t, args...)
		if err != nil {
			t.Fatalf("auth %v error = %v", args, err)
		}
		if strings.Contains(stdout+stderr, secret) {
			t.Fatalf("auth %v leaked the credential", args)
		}
	}

	if _, _, err := executeAuthCommand(t, "remove", "--profile", "office"); err != nil {
		t.Fatalf("auth remove error = %v", err)
	}
	if _, err := credential.NewStore(credentialDir).Get("shared"); err == nil {
		t.Fatal("unreferenced credential still exists after profile removal")
	}
	p, err := profiles.Get("office")
	if err != nil || p.Credential != "" {
		t.Fatalf("profile credential not cleared: %#v err=%v", p, err)
	}
}

func TestAuthRemoveProtectsSharedCredential(t *testing.T) {
	profilePath, credentialDir := setupAuthCommandTest(t)
	profiles := profile.NewStore(profilePath)
	for _, name := range []string{"office", "backup"} {
		if err := profiles.Set(name, name+":22", ""); err != nil {
			t.Fatal(err)
		}
		if err := profiles.SetCredential(name, "shared"); err != nil {
			t.Fatal(err)
		}
	}
	if err := credential.NewStore(credentialDir).Set("shared", "tskey-auth-shared-secret", false); err != nil {
		t.Fatal(err)
	}

	if _, _, err := executeAuthCommand(t, "remove", "shared"); err == nil {
		t.Fatal("removing a referenced credential should require --force")
	}
	if _, _, err := executeAuthCommand(t, "remove", "shared", "--force"); err != nil {
		t.Fatalf("forced shared removal error = %v", err)
	}
	for _, name := range []string{"office", "backup"} {
		p, err := profiles.Get(name)
		if err != nil || p.Credential != "" {
			t.Fatalf("profile %s still references shared: %#v err=%v", name, p, err)
		}
	}
}

func TestAuthSetRejectsHeadscaleKeyForSaaSProfile(t *testing.T) {
	profilePath, _ := setupAuthCommandTest(t)
	if err := profile.NewStore(profilePath).Set("office", "host:22", ""); err != nil {
		t.Fatal(err)
	}
	authSecretReader = func(string) (string, error) { return "hskey-auth-wrong-plane", nil }

	_, _, err := executeAuthCommand(t, "set", "--profile", "office")
	if err == nil || !strings.Contains(err.Error(), "custom control URL") {
		t.Fatalf("auth set error = %v, want control-plane mismatch", err)
	}
}

func TestAuthDefaultsUsePerUserPaths(t *testing.T) {
	if defaultCredentialStoreDir != config.CredentialStoreDir() {
		t.Fatalf("default credential dir = %q, want %q", defaultCredentialStoreDir, config.CredentialStoreDir())
	}
}
