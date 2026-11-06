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
