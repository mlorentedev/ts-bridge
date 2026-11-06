//go:build windows

package secretfile

import (
	"fmt"
	"os/exec"

	"golang.org/x/sys/windows"
)

var runICACLS = func(args ...string) error {
	return exec.Command("icacls", args...).Run()
}

// currentUserSID returns the SID of the user the process token runs as. The
// grant names that SID, not %USERNAME%: the name is ambiguous between a local
// and a domain account, any process can set it, and a service has none (#632).
var currentUserSID = func() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

// Harden restricts path to its owner: mode 0600 (0700 for a directory) on Unix,
// an ACL granting only the process token's user on Windows.
func Harden(path string, directory bool) error {
	sid, err := currentUserSID()
	if err != nil {
		return fmt.Errorf("read the process token's user: %w", err)
	}
	grant := "*" + sid + ":F"
	if directory {
		grant = "*" + sid + ":(OI)(CI)F"
	}
	if err := runICACLS(path, "/inheritance:r", "/grant:r", grant); err != nil {
		return fmt.Errorf("icacls: %w", err)
	}
	return nil
}
