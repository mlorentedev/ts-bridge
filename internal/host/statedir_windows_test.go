//go:build windows

package host

import (
	"errors"
	"os"
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
		{"users may read", "O:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FR;;;BU)", ""},
		{"a deny ACE is fine", "O:BAD:P(D;;FA;;;BU)(A;;FA;;;SY)", ""},
		{"inherit-only ACE does not apply here", "O:BAD:P(A;;FA;;;SY)(A;OICIIO;FA;;;CO)", ""},
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
			err = checkWindowsDACL(`C:\ProgramData\ts-bridge`, sd)
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

// A directory the current user created is theirs, so it is refused.
func TestCheckStateDir_Windows(t *testing.T) {
	if err := CheckStateDir(t.TempDir()); err == nil {
		t.Error("CheckStateDir(user temp dir) = nil, want it refused")
	}
	if err := CheckStateDir(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("CheckStateDir(absent) = nil, want an error")
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
	if errors.Is(err, windows.ERROR_INVALID_OWNER) || errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
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
