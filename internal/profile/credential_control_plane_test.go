package profile

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSetCredentialRefusesASharedNameOnAnotherControlPlane(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.yaml"))
	if err := store.Set("a", "host-a:22", "https://p.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("b", "host-b:22", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCredential("a", "k"); err != nil {
		t.Fatal(err)
	}
	err := store.SetCredential("b", "k")
	var shared *CredentialSharedError
	if !errors.As(err, &shared) || shared.Profile != "a" {
		t.Fatalf("SetCredential(b, k) = %v, want a CredentialSharedError naming profile a", err)
	}
	if err := store.CheckCredentialShare("b", "k"); !errors.As(err, &shared) {
		t.Fatalf("CheckCredentialShare(b, k) = %v, want a CredentialSharedError", err)
	}
	if err := store.CheckCredentialShare("a", "k"); err != nil {
		t.Fatalf("CheckCredentialShare(a, k) = %v, want nil for the profile's own reference", err)
	}
}
