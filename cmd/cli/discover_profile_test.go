package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"ts-bridge/internal/profile"
)

// discover --profile NAME keeps the chosen host as a named profile, the
// current connection model (ADR-012), and leaves ./.env alone (#646).
func TestDiscoverSavesTheSelectionAsAProfile(t *testing.T) {
	t.Chdir(t.TempDir())
	storePath := filepath.Join(t.TempDir(), "profiles.yaml")
	old := defaultProfileStorePath
	defaultProfileStorePath = storePath
	t.Cleanup(func() { defaultProfileStorePath = old })

	flags := discoverFlags{Port: 3390, Profile: "office"}
	if err := persistSelection(&cobra.Command{}, flags, "acemagic", "https://hs.example.com"); err != nil {
		t.Fatalf("persistSelection() = %v", err)
	}
	got, err := profile.NewStore(storePath).Get("office")
	if err != nil {
		t.Fatalf("profile office: %v", err)
	}
	if got.Target != "acemagic:3390" || got.ControlURL != "https://hs.example.com" {
		t.Errorf("profile = %+v, want target acemagic:3390 on https://hs.example.com", got)
	}
	if _, err := os.Stat(".env"); !os.IsNotExist(err) {
		t.Errorf(".env exists after discover --profile (stat err %v)", err)
	}
}

func TestDiscoverProfileFlagIsRegistered(t *testing.T) {
	cmd := newDiscoverCmd()
	if cmd.Flags().Lookup("profile") == nil {
		t.Fatal("discover has no --profile flag")
	}
	if !strings.Contains(cmd.Long, "--profile") {
		t.Errorf("discover help does not mention --profile:\n%s", cmd.Long)
	}
}
