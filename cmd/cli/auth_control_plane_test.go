package cmd

import (
	"strings"
	"testing"

	"ts-bridge/internal/credential"
	"ts-bridge/internal/profile"
)

// A credential name shared by profiles on two control planes would carry a key
// issued for one plane to the other (#653, scenario 1).
func TestAuthSetRefusesToShareACredentialAcrossControlPlanes(t *testing.T) {
	profilePath, credentialDir := setupAuthCommandTest(t)
	profiles := profile.NewStore(profilePath)
	if err := profiles.Set("a", "host-a:22", "https://p.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := profiles.Set("b", "host-b:22", "https://q.example.com"); err != nil {
		t.Fatal(err)
	}
	const original = "tskey-auth-forp-1"    // #nosec G101 -- synthetic test credential
	const replacement = "tskey-auth-forq-1" // #nosec G101 -- synthetic test credential
	authSecretReader = func(string) (string, error) { return original, nil }
	if _, _, err := executeAuthCommand(t, "set", "k", "--profile", "a"); err != nil {
		t.Fatalf("auth set k --profile a: %v", err)
	}

	authSecretReader = func(string) (string, error) { return replacement, nil }
	_, _, err := executeAuthCommand(t, "set", "k", "--profile", "b", "--force")
	if err == nil || !strings.Contains(err.Error(), `"a"`) || !strings.Contains(err.Error(), "p.example.com") {
		t.Fatalf("auth set k --profile b --force error = %v, want a refusal naming profile a and its control plane", err)
	}
	if got, _ := credential.NewStore(credentialDir).Get("k"); got != original {
		t.Fatalf("credential k = %q after the refusal, want the original key kept", got)
	}
	if b, _ := profiles.Get("b"); b.Credential != "" {
		t.Fatalf("profile b references %q after the refusal, want no reference", b.Credential)
	}
}

// A control URL rewritten between the read and the association must not be
// adopted: runAuthSet associates against the URL it read first (#653, scenario 2).
func TestAuthSetRefusesWhenTheControlURLChangesMidWrite(t *testing.T) {
	profilePath, _ := setupAuthCommandTest(t)
	profiles := profile.NewStore(profilePath)
	if err := profiles.Set("a", "host-a:22", "https://p.example.com"); err != nil {
		t.Fatal(err)
	}
	authSecretReader = func(string) (string, error) {
		if err := profiles.Set("a", "host-a:22", "https://q.example.com"); err != nil {
			t.Fatal(err)
		}
		return "tskey-auth-race-1", nil // #nosec G101 -- synthetic test credential
	}
	if _, _, err := executeAuthCommand(t, "set", "k", "--profile", "a", "--force"); err == nil {
		t.Fatal("auth set k --profile a succeeded after the control URL changed, want a refusal")
	}
	if a, _ := profiles.Get("a"); a.Credential != "" {
		t.Fatalf("profile a references %q after the refusal, want no reference", a.Credential)
	}
}

func TestAuthSetSharesACredentialOnOneControlPlane(t *testing.T) {
	profilePath, _ := setupAuthCommandTest(t)
	profiles := profile.NewStore(profilePath)
	for _, name := range []string{"a", "b"} {
		if err := profiles.Set(name, "host-"+name+":22", "https://P.example.com/"); err != nil {
			t.Fatal(err)
		}
	}
	authSecretReader = func(string) (string, error) { return "tskey-auth-shared-1", nil }
	if _, _, err := executeAuthCommand(t, "set", "k", "--profile", "a"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeAuthCommand(t, "set", "k", "--profile", "b", "--force"); err != nil {
		t.Fatalf("sharing k on one control plane: %v", err)
	}
}
