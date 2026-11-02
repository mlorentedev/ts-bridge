//go:build windows

package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileDeleteChild is FILE_DELETE_CHILD, which x/sys/windows does not export.
const fileDeleteChild = 0x40

// stateDirWriteMask is every right that lets a holder change the directory,
// its contents or its permissions.
const stateDirWriteMask = windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.FILE_WRITE_EA |
	fileDeleteChild | windows.FILE_WRITE_ATTRIBUTES | windows.DELETE | windows.WRITE_DAC |
	windows.WRITE_OWNER | windows.GENERIC_WRITE | windows.GENERIC_ALL

// stateDirSDDL owns the directory by Administrators and grants full control to
// SYSTEM and Administrators only, with inheritance from the parent cut.
const stateDirSDDL = "O:BAD:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"

// CheckStateDir refuses a state directory that a SID other than SYSTEM or
// Administrators owns or may write. A link is refused rather than followed.
func CheckStateDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("state_dir %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("state_dir %s: not a directory", path)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("state_dir %s: read its permissions: %w", path, err)
	}
	return checkWindowsDACL(path, sd)
}

// EnsureStateDir creates an absent state directory with stateDirSDDL, then
// checks it. An existing directory is checked, never repaired.
func EnsureStateDir(path string) error {
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create state_dir %s: %w", path, err)
		}
		if err := protectStateDir(path); err != nil {
			return fmt.Errorf("protect state_dir %s: %w", path, err)
		}
	}
	return CheckStateDir(path)
}

func protectStateDir(path string) error {
	sd, err := windows.SecurityDescriptorFromString(stateDirSDDL)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner, nil, dacl, nil)
}

func checkWindowsDACL(path string, sd *windows.SECURITY_DESCRIPTOR) error {
	owner, _, err := sd.Owner()
	if err != nil {
		return fmt.Errorf("state_dir %s: read its owner: %w", path, err)
	}
	if !trustedSID(owner) {
		return fmt.Errorf("state_dir %s: owned by %s, want SYSTEM or Administrators", path, owner)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return fmt.Errorf("state_dir %s: no DACL, so everyone has full access", path)
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return fmt.Errorf("state_dir %s: read ACE %d: %w", path, i, err)
		}
		if err := checkACE(path, ace); err != nil {
			return err
		}
	}
	return nil
}

// checkACE refuses an ACE that lets an untrusted SID write. Deny ACEs and
// inherit-only ones, which do not apply to the directory itself, pass; any
// other ACE type is refused rather than guessed at.
func checkACE(path string, ace *windows.ACCESS_ALLOWED_ACE) error {
	if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
		return nil
	}
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
		return fmt.Errorf("state_dir %s: ACE type %d is not supported; grant SYSTEM and Administrators only", path, ace.Header.AceType)
	}
	// #nosec G103 -- the SID starts at SidStart inside the ACE that GetAce returned.
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if ace.Mask&stateDirWriteMask != 0 && !trustedSID(sid) {
		return fmt.Errorf("state_dir %s: %s can write it; only SYSTEM and Administrators may", path, sid)
	}
	return nil
}

func trustedSID(sid *windows.SID) bool {
	return sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid)
}
