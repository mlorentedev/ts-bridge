//go:build windows

package secretfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestHardenLeavesOnlyTheTokenUserInTheDACL hardens a real file and directory
// and reads the DACL back: one explicit ACE, for the process token's user SID.
// USERNAME names another account that exists on every Windows install, so a
// grant by name lands on the wrong principal (#632). Each path first carries an
// explicit SYSTEM ACE, as files on a GitHub-hosted runner do: removing
// inheritance alone leaves such an entry in place.
func TestHardenLeavesOnlyTheTokenUserInTheDACL(t *testing.T) {
	t.Setenv("USERNAME", "Guest")
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "store")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		path      string
		directory bool
		flags     uint8
	}{
		{"file", file, false, 0},
		{"directory", dir, true, windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// #nosec G204 -- fixed arguments on the test's own temp path.
			if out, err := exec.Command("icacls", tt.path, "/grant", "*S-1-5-18:F").CombinedOutput(); err != nil {
				t.Fatalf("seed a SYSTEM ACE: %v: %s", err, out)
			}
			if err := Harden(tt.path, tt.directory); err != nil {
				t.Fatal(err)
			}
			ace := onlyACE(t, tt.path)
			sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
			if !sid.Equals(user.User.Sid) {
				t.Fatalf("ACE grants %s, want the token user %s", sid, user.User.Sid)
			}
			if ace.Header.AceFlags != tt.flags {
				t.Fatalf("ACE flags = %#x, want %#x (explicit; inherited by children only for a directory)", ace.Header.AceFlags, tt.flags)
			}
			if ace.Mask != fileAllAccess {
				t.Fatalf("ACE mask = %#x, want full control %#x", ace.Mask, fileAllAccess)
			}
		})
	}
}

// onlyACE reads path's DACL, requires it to be protected and to hold exactly
// one ACE, and returns that ACE.
func onlyACE(t *testing.T, path string) *windows.ACCESS_ALLOWED_ACE {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("DACL %v is not protected from inheritance", sd)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if dacl == nil || dacl.AceCount != 1 {
		t.Fatalf("DACL %v has %d ACEs, want exactly one", sd, aceCount(dacl))
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	return ace
}

func aceCount(dacl *windows.ACL) uint16 {
	if dacl == nil {
		return 0
	}
	return dacl.AceCount
}
