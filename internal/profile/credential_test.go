package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreCredentialReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	store := NewStore(path)
	if err := store.Set("office", "host:45000", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCredential("office", "office-key"); err != nil {
		t.Fatalf("SetCredential() error = %v", err)
	}

	got, err := store.Get("office")
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != "office-key" {
		t.Fatalf("Credential = %q, want office-key", got.Credential)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "credential: office-key") {
		t.Fatalf("profile YAML missing credential reference:\n%s", text)
	}
	if strings.Contains(text, "tskey-") || strings.Contains(text, "hskey-") || strings.Contains(text, ".key") {
		t.Fatal("profile YAML contains secret material or a credential file path")
	}
}

func TestStoreCredentialReferenceIsBackwardCompatibleAndPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	legacy := "descriptor_version: 1\nprofiles:\n  office:\n    target: host:45000\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	got, err := store.Get("office")
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != "" {
		t.Fatalf("legacy profile Credential = %q, want empty", got.Credential)
	}

	if err := store.SetCredential("office", "shared"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("office", "new-host:45000", "https://vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	got, err = store.Get("office")
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != "shared" {
		t.Fatalf("Set() discarded Credential: %#v", got)
	}
	if err := store.Import("office", Descriptor{Host: "imported", Port: 22}); err != nil {
		t.Fatal(err)
	}
	got, err = store.Get("office")
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != "shared" {
		t.Fatalf("Import() discarded Credential: %#v", got)
	}
}

func TestStoreClearCredentialAndListProfiles(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.yaml"))
	if err := store.Set("office", "host:45000", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCredential("office", "office-key"); err != nil {
		t.Fatal(err)
	}
	profiles, err := store.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if profiles["office"].Credential != "office-key" {
		t.Fatalf("ListProfiles() = %#v", profiles)
	}
	if err := store.ClearCredential("office"); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("office")
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != "" {
		t.Fatalf("ClearCredential left %q", got.Credential)
	}
}

func TestStoreSetCredentialValidatesProfileAndCredential(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.yaml"))
	if err := store.SetCredential("missing", "office"); err == nil {
		t.Fatal("SetCredential() expected missing-profile error")
	}
	if err := store.Set("office", "host:45000", ""); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../key", "two words", ".hidden"} {
		if err := store.SetCredential("office", name); err == nil {
			t.Fatalf("SetCredential(%q) expected validation error", name)
		}
	}
}
