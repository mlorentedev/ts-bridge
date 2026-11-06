//go:build windows

package secretfile

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// fileAllAccess is FILE_ALL_ACCESS, the mask icacls shows as (F).
const fileAllAccess = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1FF

// Harden restricts path to its owner: mode 0600 (0700 for a directory) on Unix,
// on Windows a protected DACL holding one ACE, for the process token's user.
//
// The ACE names the token's SID, not %USERNAME%: the name is ambiguous between
// a local and a domain account, any process can set it, and a service has none
// (#632). The DACL is written whole rather than edited with icacls, because
// removing inheritance keeps explicit entries such as the SYSTEM and
// Administrators grants a file can already carry.
func Harden(path string, directory bool) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("read the process token's user: %w", err)
	}
	inheritance := uint32(windows.NO_INHERITANCE)
	if directory {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: fileAllAccess,
		AccessMode:        windows.SET_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return fmt.Errorf("build the owner-only ACL: %w", err)
	}
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, info, nil, nil, acl, nil); err != nil {
		return fmt.Errorf("set the owner-only ACL on %s: %w", path, err)
	}
	return nil
}
