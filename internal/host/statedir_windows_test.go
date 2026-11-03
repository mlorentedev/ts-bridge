//go:build windows

package host

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCheckWindowsDACL(t *testing.T) {
	tests := []struct {
		name, sddl, wantErr string // wantErr "" accepts
	}{
		{"SYSTEM and Administrators only", "O:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)", ""},
		{"owned by SYSTEM", "O:SYD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)", ""},
		{"a deny ACE is fine", "O:BAD:P(D;;FA;;;BU)(A;;FA;;;SY)", ""},
		{"creator owner on children is the creator", "O:BAD:P(A;;FA;;;SY)(A;OICIIO;FA;;;CO)", ""},
		{"users may read", "O:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FR;;;BU)", "S-1-5-32-545"},
		{"users read what is inside", "O:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1200a9;;;BU)", "S-1-5-32-545"},
		{"users inherit full control", "O:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICIIO;FA;;;BU)", "S-1-5-32-545"},
		{"users may write", "O:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FW;;;BU)", "S-1-5-32-545"},
		{"everyone may add files", "O:BAD:P(A;;0x2;;;WD)", "S-1-1-0"},
		{"generic write", "O:BAD:P(A;;GW;;;AU)", "S-1-5-11"},
		{"change permissions", "O:BAD:P(A;;WD;;;AU)", "S-1-5-11"},
		{"delete", "O:BAD:P(A;;SD;;;AU)", "S-1-5-11"},
		{"owned by users", "O:BUD:P(A;;FA;;;SY)", "owned by S-1-5-32-545"},
		{"null DACL", "O:BAD:NO_ACCESS_CONTROL", "no DACL"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sd, err := windows.SecurityDescriptorFromString(tc.sddl)
			if err != nil {
				t.Fatalf("SDDL %q: %v", tc.sddl, err)
			}
			err = checkWindowsDACL("state_dir", `C:\ProgramData\ts-bridge`, sd)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("checkWindowsDACL() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), `C:\ProgramData\ts-bridge`) {
				t.Errorf("checkWindowsDACL() = %v, want an error naming the path and %q", err, tc.wantErr)
			}
		})
	}
}

// A directory the current user created is refused for that user: as its owner
// (unelevated) or through its inherited ACE (elevated tokens default the owner
// to Administrators). Either way the real GetNamedSecurityInfo read reached
// the DACL check.
func TestCheckStateDir_Windows(t *testing.T) {
	err := CheckStateDir(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "S-1-5-21-") ||
		!(strings.Contains(err.Error(), "owned by") || strings.Contains(err.Error(), "can access it")) {
		t.Errorf("CheckStateDir(user temp dir) = %v, want it refused for the user's SID", err)
	}
	absent := filepath.Join(t.TempDir(), "absent")
	if err := CheckStateDir(absent); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("CheckStateDir(absent) = %v, want a not-exist error", err)
	}
}

func TestEnsureStateDir(t *testing.T) {
	loose := t.TempDir()
	if err := EnsureStateDir(loose); err == nil {
		t.Error("EnsureStateDir(existing user dir) = nil, want it refused, not repaired")
	}
	if err := CheckStateDir(loose); err == nil {
		t.Error("EnsureStateDir repaired an existing directory")
	}

	dir := filepath.Join(t.TempDir(), "serve")
	err := EnsureStateDir(dir)
	if !windows.GetCurrentProcessToken().IsElevated() {
		if err == nil {
			t.Fatal("EnsureStateDir succeeded unelevated; it must not be able to own the directory by Administrators")
		}
		t.Skipf("owning a directory by Administrators needs an elevated token: %v", err)
	}
	if err != nil {
		t.Fatalf("EnsureStateDir(absent) = %v, want it created protected", err)
	}
	if err := CheckStateDir(dir); err != nil {
		t.Errorf("created state dir fails the check: %v", err)
	}
	if err := EnsureStateDir(dir); err != nil {
		t.Errorf("second EnsureStateDir = %v, want nil (idempotent)", err)
	}
	_ = os.RemoveAll(dir)
}

// A junction above state_dir (a standard user can plant one) is refused by
// both functions, and EnsureStateDir creates nothing through it.
func TestCheckStateDir_Junction(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "stash")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(dir, "ts-bridge")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("mklink /J: %v %s", err, out)
	}
	path := filepath.Join(junction, "serve")
	for name, fn := range map[string]func(string) error{"CheckStateDir": CheckStateDir, "EnsureStateDir": EnsureStateDir} {
		if err := fn(path); err == nil || !strings.Contains(err.Error(), "link or junction") {
			t.Errorf("%s(under a junction) = %v, want it refused naming the junction", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "serve")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("EnsureStateDir created state_dir through the junction: %v", err)
	}
}
