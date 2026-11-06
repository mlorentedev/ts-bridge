package profile

import (
	"errors"
	"path/filepath"
	"strings"
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

// The association re-reads the profile: a control_url changed since auth set
// read it must not inherit the key (#653, scenario 2).
func TestAssociateCredentialVerifiesTheControlURLAtWriteTime(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.yaml"))
	if err := store.Set("a", "host-a:22", "https://p.example.com"); err != nil {
		t.Fatal(err)
	}
	read, err := store.Get("a")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("a", "host-a:22", "https://q.example.com"); err != nil {
		t.Fatal(err)
	}
	err = store.AssociateCredential("a", "k", read.ControlURL)
	if err == nil || !strings.Contains(err.Error(), "q.example.com") {
		t.Fatalf("AssociateCredential() = %v, want a refusal naming the new control plane", err)
	}
	if got, _ := store.Get("a"); got.Credential != "" {
		t.Fatalf("profile a references %q, want no reference", got.Credential)
	}
	if err := store.AssociateCredential("a", "k", "https://Q.example.com/"); err != nil {
		t.Fatalf("AssociateCredential() with the current control plane = %v", err)
	}
}
