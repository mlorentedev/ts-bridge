package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"ts-bridge/internal/config"
	"ts-bridge/internal/profile"
)

// TestApplyProfile_ResolvesTargetFromStore verifies that applyProfile fills
// cfg.Target from the store when no explicit target was provided.
func TestApplyProfile_ResolvesTargetFromStore(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.yaml")

	s := profile.NewStore(storePath)
	d := profile.Descriptor{Host: "acemagic-office", Port: 45000, ControlURL: ""}
	if err := s.Import("home", d); err != nil {
		t.Fatalf("Import: %v", err)
	}

	cfg := config.Config{} // no target set
	if err := applyProfile(&cfg, "home", storePath); err != nil {
		t.Fatalf("applyProfile: %v", err)
	}
	if cfg.Target != "acemagic-office:45000" {
		t.Errorf("Target = %q, want %q", cfg.Target, "acemagic-office:45000")
	}
}

// TestApplyProfile_SetsControlURLForHeadscale verifies that a profile with a
// Headscale control URL also populates cfg.ControlURL.
func TestApplyProfile_SetsControlURLForHeadscale(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.yaml")

	s := profile.NewStore(storePath)
	d := profile.Descriptor{Host: "host", Port: 3389, ControlURL: "https://vpn.example.com"}
	if err := s.Import("work", d); err != nil {
		t.Fatalf("Import: %v", err)
	}

	cfg := config.Config{}
	if err := applyProfile(&cfg, "work", storePath); err != nil {
		t.Fatalf("applyProfile: %v", err)
	}
	if cfg.ControlURL != "https://vpn.example.com" {
		t.Errorf("ControlURL = %q, want %q", cfg.ControlURL, "https://vpn.example.com")
	}
}

// TestApplyProfile_ExplicitTargetWins verifies backward compat: if Target is
// already set (from flag/env/yaml), --profile does not override it.
func TestApplyProfile_ExplicitTargetWins(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.yaml")

	s := profile.NewStore(storePath)
	if err := s.Import("home", profile.Descriptor{Host: "profile-host", Port: 45000}); err != nil {
		t.Fatalf("Import: %v", err)
	}

	cfg := config.Config{Target: "explicit-host:3389"} // already set
	if err := applyProfile(&cfg, "home", storePath); err != nil {
		t.Fatalf("applyProfile: %v", err)
	}
	if cfg.Target != "explicit-host:3389" {
		t.Errorf("explicit Target should win; got %q", cfg.Target)
	}
}

// TestApplyProfile_UnknownProfileReturnsError ensures a missing profile fails
// with a clear error rather than silently using no target.
func TestApplyProfile_UnknownProfileReturnsError(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.yaml")

	cfg := config.Config{}
	err := applyProfile(&cfg, "nonexistent", storePath)
	if err == nil {
		t.Error("expected error for unknown profile, got nil")
	}
}

// TestApplyProfile_EmptyName_NoOp verifies that passing an empty profile name
// is a no-op (no flag provided case).
func TestApplyProfile_EmptyName_NoOp(t *testing.T) {
	cfg := config.Config{Target: "host:3389"}
	if err := applyProfile(&cfg, "", ""); err != nil {
		t.Fatalf("empty name should be a no-op, got: %v", err)
	}
	if cfg.Target != "host:3389" {
		t.Errorf("Target modified unexpectedly: %q", cfg.Target)
	}
}

// TestImportCommandRegistered verifies the import subcommand is wired into root.
func TestImportCommandRegistered(t *testing.T) {
	found := false
	for _, sub := range NewRootCmd().Commands() {
		if sub.Name() == "import" {
			found = true
			break
		}
	}
	if !found {
		t.Error("import subcommand not registered on production root")
	}
}

// TestRunImport_WritesProfile exercises the import command end-to-end
// with a temp store path injected via the package-level var.
func TestRunImport_WritesProfile(t *testing.T) {
	dir := t.TempDir()
	origPath := defaultProfileStorePath
	defaultProfileStorePath = filepath.Join(dir, "profiles.yaml")
	defer func() { defaultProfileStorePath = origPath }()

	storePath := defaultProfileStorePath
	if err := runImportDirect("home", "tsb://acemagic-office:45000?cp=saas", storePath); err != nil {
		t.Fatalf("runImportDirect: %v", err)
	}

	s := profile.NewStore(storePath)
	p, err := s.Get("home")
	if err != nil {
		t.Fatalf("Get after import: %v", err)
	}
	if p.Target != "acemagic-office:45000" {
		t.Errorf("Target = %q, want %q", p.Target, "acemagic-office:45000")
	}
}

// TestRunImport_MalformedDescriptor ensures a bad descriptor is rejected cleanly.
func TestRunImport_MalformedDescriptor(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.yaml")
	err := runImportDirect("home", "not-a-descriptor", storePath)
	if err == nil {
		t.Error("expected error for malformed descriptor")
	}
	// No profile file should have been written.
	if _, statErr := os.Stat(storePath); !os.IsNotExist(statErr) {
		t.Error("profiles.yaml should not exist after failed import")
	}
}

// TestControlPlaneParity_HeadscaleDescriptorRoundTrip verifies that a descriptor
// carrying a Headscale control URL imports and resolves identically to a SaaS
// descriptor — only ControlURL differs, not the target.
func TestControlPlaneParity_HeadscaleDescriptorRoundTrip(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.yaml")

	const headscaleURL = "https://vpn.corp.example.com"
	descriptor := "tsb://rdp.corp.example.com:3389?cp=" + headscaleURL

	if err := runImportDirect("work", descriptor, storePath); err != nil {
		t.Fatalf("runImportDirect with Headscale URL: %v", err)
	}

	cfg := config.Config{}
	if err := applyProfile(&cfg, "work", storePath); err != nil {
		t.Fatalf("applyProfile: %v", err)
	}
	if cfg.Target != "rdp.corp.example.com:3389" {
		t.Errorf("Target = %q, want %q", cfg.Target, "rdp.corp.example.com:3389")
	}
	if cfg.ControlURL != headscaleURL {
		t.Errorf("ControlURL = %q, want %q", cfg.ControlURL, headscaleURL)
	}
}

// TestBackwardCompat_ExistingTargetWithoutProfile verifies that a non-empty
// cfg.Target from --target / TS_TARGET is honored unchanged when no profile
// name is given — the legacy one-env-var connect path must not regress.
func TestBackwardCompat_ExistingTargetWithoutProfile(t *testing.T) {
	cfg := config.Config{Target: "100.64.0.1:3389"}
	if err := applyProfile(&cfg, "", ""); err != nil {
		t.Fatalf("applyProfile with empty name should be no-op: %v", err)
	}
	if cfg.Target != "100.64.0.1:3389" {
		t.Errorf("Target should be unchanged; got %q", cfg.Target)
	}
	if cfg.ControlURL != "" {
		t.Errorf("ControlURL should be empty; got %q", cfg.ControlURL)
	}
}
