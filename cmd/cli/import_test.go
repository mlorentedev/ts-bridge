package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/profile"
)

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
	if err := runImportDirect("home", "tsb://acemagic-office:45000?cp=saas", storePath, false); err != nil {
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
	err := runImportDirect("home", "not-a-descriptor", storePath, false)
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

	if err := runImportDirect("work", descriptor, storePath, false); err != nil {
		t.Fatalf("runImportDirect with Headscale URL: %v", err)
	}

	got, err := profile.NewStore(storePath).Get("work")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Target != "rdp.corp.example.com:3389" {
		t.Errorf("Target = %q, want %q", got.Target, "rdp.corp.example.com:3389")
	}
	if got.ControlURL != headscaleURL {
		t.Errorf("ControlURL = %q, want %q", got.ControlURL, headscaleURL)
	}
}

// import is the first step of import -> auth set -> connect; it must say what
// comes next, since the profile cannot connect without a credential.
func TestRunImport_PrintsNextSteps(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "profiles.yaml")
	stdout, _ := captureInitOutput(t, func() {
		if err := runImportDirect("home", "tsb://acemagic-office:45000?cp=saas", storePath, false); err != nil {
			t.Errorf("runImportDirect: %v", err)
		}
	})
	for _, want := range []string{"ts-bridge auth set --profile home", "ts-bridge connect --profile home"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q; want it to contain %q", stdout, want)
		}
	}
}

// A malformed descriptor reported its prefix twice: the parser's own
// "invalid descriptor ..." wrapped again by the command.
func TestImportErrorIsNotDoublePrefixed(t *testing.T) {
	setupAuthCommandTest(t)
	cmd := newImportCmd()
	cmd.SetArgs([]string{"home", "tsb://bad"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	err := cmd.Execute()
	if err == nil {
		t.Fatal("import of tsb://bad succeeded")
	}
	if n := strings.Count(err.Error(), "invalid descriptor"); n != 1 {
		t.Fatalf("error = %q; 'invalid descriptor' appears %d times, want 1", err, n)
	}
}

// Cobra's arity error alone ("accepts 2 arg(s), received 0") does not say what
// the two arguments are; the usage line does.
func TestImportWithoutArgsPrintsUsageLine(t *testing.T) {
	cmd := newImportCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	err := cmd.Execute()
	if err == nil {
		t.Fatal("import with no arguments succeeded")
	}
	if !strings.Contains(err.Error(), "import <name> <descriptor>") {
		t.Fatalf("error = %q; want the usage line `import <name> <descriptor>`", err)
	}
}
