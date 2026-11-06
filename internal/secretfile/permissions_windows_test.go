//go:build windows

package secretfile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestHardenGrantsTheTokenUserSID(t *testing.T) {
	t.Setenv("USERNAME", "not-the-token-user")
	originalRun, originalSID := runICACLS, currentUserSID
	t.Cleanup(func() { runICACLS, currentUserSID = originalRun, originalSID })

	currentUserSID = func() (string, error) { return "S-1-5-21-1-2-3-1001", nil }
	var calls [][]string
	runICACLS = func(args ...string) error {
		calls = append(calls, append([]string(nil), args...))
		return nil
	}

	if err := Harden(`C:\store`, true); err != nil {
		t.Fatal(err)
	}
	if err := Harden(`C:\store\office.key`, false); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{`C:\store`, "/inheritance:r", "/grant:r", "*S-1-5-21-1-2-3-1001:(OI)(CI)F"},
		{`C:\store\office.key`, "/inheritance:r", "/grant:r", "*S-1-5-21-1-2-3-1001:F"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("icacls calls = %#v, want %#v", calls, want)
	}
}

// TestHardenLeavesOnlyTheTokenUserInTheDACL runs the real icacls and reads the
// resulting DACL back: one explicit ACE, for the process token's user SID.
// USERNAME names another account that exists on every Windows install, so a
// grant by name lands on the wrong principal (#632).
func TestHardenLeavesOnlyTheTokenUserInTheDACL(t *testing.T) {
	t.Setenv("USERNAME", "Guest")
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Harden(path, false); err != nil {
		t.Fatal(err)
	}

	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
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
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !sid.Equals(user.User.Sid) {
		t.Fatalf("ACE grants %s, want the token user %s", sid, user.User.Sid)
	}
	if ace.Header.AceFlags&windows.INHERITED_ACE != 0 {
		t.Fatalf("ACE for %s is inherited, want explicit", sid)
	}
}

func aceCount(dacl *windows.ACL) uint16 {
	if dacl == nil {
		return 0
	}
	return dacl.AceCount
}
