//go:build windows

package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// stateDirSDDL owns the directory by Administrators and grants full control to
// SYSTEM and Administrators only, with inheritance from the parent cut.
const stateDirSDDL = "O:BAD:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"

// CheckStateDir refuses a state directory that a SID other than SYSTEM or
// Administrators owns or may access, or that sits behind a link or junction.
func CheckStateDir(path string) error {
	if err := checkNoReparse(path); err != nil {
		return err
	}
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
	return checkWindowsDACL("state_dir", path, sd)
}

// EnsureStateDir creates an absent state directory with stateDirSDDL, then
// checks it. An existing directory is checked, never repaired. A junction or
// link above it is refused before anything is created through it.
func EnsureStateDir(path string) error {
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		if err := checkNoReparse(filepath.Dir(path)); err != nil {
			return err
		}
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

func checkWindowsDACL(what, path string, sd *windows.SECURITY_DESCRIPTOR) error {
	owner, _, err := sd.Owner()
	if err != nil {
		return fmt.Errorf("%s %s: read its owner: %w", what, path, err)
	}
	if !trustedSID(owner) {
		return fmt.Errorf("%s %s: owned by %s, want SYSTEM or Administrators", what, path, owner)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return fmt.Errorf("%s %s: no DACL, so everyone has full access", what, path)
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return fmt.Errorf("%s %s: read ACE %d: %w", what, path, i, err)
		}
		if err := checkACE(what, path, ace); err != nil {
			return err
		}
	}
	return nil
}

// checkACE refuses an allow ACE that grants an untrusted SID any access, to
// the directory or, through inheritance, to the node key inside it. CREATOR
// OWNER on an inherit-only ACE resolves to whoever creates the child (serve),
// so it passes; so do deny ACEs. Any other ACE type is refused, not guessed at.
func checkACE(what, path string, ace *windows.ACCESS_ALLOWED_ACE) error {
	if ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
		return nil
	}
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
		return fmt.Errorf("%s %s: ACE type %d is not supported; grant SYSTEM and Administrators only", what, path, ace.Header.AceType)
	}
	// #nosec G103 -- the SID starts at SidStart inside the ACE that GetAce returned.
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	inheritOnly := ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0
	if ace.Mask == 0 || trustedSID(sid) || (inheritOnly && sid.IsWellKnown(windows.WinCreatorOwnerSid)) {
		return nil
	}
	return fmt.Errorf("%s %s: %s can access it or what it holds; only SYSTEM and Administrators may", what, path, sid)
}

func trustedSID(sid *windows.SID) bool {
	return sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid)
}

// checkNoReparse refuses path when it or an existing ancestor is a symlink or
// a junction (Go reports a mount point as ModeIrregular): a standard user may
// plant one under ProgramData and redirect the node key into their own tree.
func checkNoReparse(path string) error {
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err == nil && info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return fmt.Errorf("state_dir %s: %s is a link or junction; use a path without one", path, p)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("state_dir %s: %w", path, err)
		}
		if filepath.Dir(p) == p {
			return nil
		}
	}
}

// CheckSecretFile refuses an auth key file that SYSTEM or Administrators do
// not own, that another SID may access, or that is reached through a link.
func CheckSecretFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("auth_key_file %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("auth_key_file %s: not a regular file", path)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("auth_key_file %s: read its permissions: %w", path, err)
	}
	return checkWindowsDACL("auth_key_file", path, sd)
}
