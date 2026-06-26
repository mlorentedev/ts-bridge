package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_ImportDescriptor_WritesProfile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "profiles.yaml"))

	d := Descriptor{Host: "acemagic-office", Port: 45000, ControlURL: ""}
	if err := s.Import("home", d); err != nil {
		t.Fatalf("Import error: %v", err)
	}

	p, err := s.Get("home")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if p.Target != "acemagic-office:45000" {
		t.Errorf("Target = %q, want %q", p.Target, "acemagic-office:45000")
	}
	if p.ControlURL != "" {
		t.Errorf("ControlURL = %q, want empty (SaaS)", p.ControlURL)
	}
}

func TestStore_Import_Headscale(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "profiles.yaml"))

	d := Descriptor{Host: "acemagic-office", Port: 45000, ControlURL: "https://vpn.example.com"}
	if err := s.Import("work", d); err != nil {
		t.Fatalf("Import error: %v", err)
	}

	p, err := s.Get("work")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if p.ControlURL != "https://vpn.example.com" {
		t.Errorf("ControlURL = %q, want %q", p.ControlURL, "https://vpn.example.com")
	}
}

func TestStore_Import_Idempotent(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "profiles.yaml"))

	d := Descriptor{Host: "host", Port: 3389, ControlURL: ""}
	for i := 0; i < 3; i++ {
		if err := s.Import("office", d); err != nil {
			t.Fatalf("Import #%d error: %v", i+1, err)
		}
	}

	// Re-read from disk to verify idempotent write.
	s2 := NewStore(filepath.Join(dir, "profiles.yaml"))
	names, err := s2.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(names) != 1 {
		t.Errorf("expected 1 profile after 3 identical imports, got %d", len(names))
	}
}

func TestStore_Get_UnknownProfile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "profiles.yaml"))
	_, err := s.Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown profile, got nil")
	}
}

func TestStore_MultipleProfiles(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "profiles.yaml"))

	imports := map[string]Descriptor{
		"home":   {Host: "host-a", Port: 3389},
		"office": {Host: "host-b", Port: 45000},
	}
	for name, d := range imports {
		if err := s.Import(name, d); err != nil {
			t.Fatalf("Import %q error: %v", name, err)
		}
	}

	names, err := s.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 profiles, got %d: %v", len(names), names)
	}
}

func TestStore_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")

	s1 := NewStore(path)
	if err := s1.Import("home", Descriptor{Host: "host", Port: 3389}); err != nil {
		t.Fatalf("Import error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected profiles.yaml to exist on disk after Import")
	}

	// New store instance reads from same path.
	s2 := NewStore(path)
	p, err := s2.Get("home")
	if err != nil {
		t.Fatalf("Get from fresh store error: %v", err)
	}
	if p.Target != "host:3389" {
		t.Errorf("Target = %q, want %q", p.Target, "host:3389")
	}
}

func TestStore_Set_WritesProfile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "profiles.yaml"))

	if err := s.Set("work", "host:3389", ""); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	p, err := s.Get("work")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if p.Target != "host:3389" {
		t.Errorf("Target = %q, want %q", p.Target, "host:3389")
	}
	if p.ControlURL != "" {
		t.Errorf("ControlURL = %q, want empty", p.ControlURL)
	}
}

func TestStore_Set_Headscale(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "profiles.yaml"))

	if err := s.Set("kubelab", "host:3389", "https://vpn.kubelab.live"); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	p, err := s.Get("kubelab")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if p.ControlURL != "https://vpn.kubelab.live" {
		t.Errorf("ControlURL = %q, want %q", p.ControlURL, "https://vpn.kubelab.live")
	}
}

func TestStore_Set_Idempotent(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "profiles.yaml"))

	for i := 0; i < 3; i++ {
		if err := s.Set("work", "host:3389", ""); err != nil {
			t.Fatalf("Set #%d error: %v", i+1, err)
		}
	}

	names, err := s.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(names) != 1 {
		t.Errorf("expected 1 profile after 3 identical sets, got %d", len(names))
	}
}

func TestStore_Set_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "profiles.yaml"))

	if err := s.Set("work", "old-host:3389", ""); err != nil {
		t.Fatalf("initial Set error: %v", err)
	}
	if err := s.Set("work", "new-host:45000", "https://vpn.example.com"); err != nil {
		t.Fatalf("overwrite Set error: %v", err)
	}

	p, err := s.Get("work")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if p.Target != "new-host:45000" {
		t.Errorf("Target = %q, want %q", p.Target, "new-host:45000")
	}
	if p.ControlURL != "https://vpn.example.com" {
		t.Errorf("ControlURL = %q, want %q", p.ControlURL, "https://vpn.example.com")
	}
}

func TestStore_Set_RejectsSecretInTarget(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "profiles.yaml"))

	cases := []string{
		"tskey-auth-abc123:3389",
		"hskey-preauth-xyz:3389",
	}
	for _, target := range cases {
		if err := s.Set("bad", target, ""); err == nil {
			t.Errorf("Set(%q) expected error for secret in target, got nil", target)
		}
	}
}
