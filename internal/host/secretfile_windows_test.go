//go:build windows

package host

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCheckWindowsSecretDACL(t *testing.T) {
	tests := []struct{ name, sddl, wantErr string }{
		{"SYSTEM and Administrators only", "O:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)", ""},
		{"users may read", "O:BAD:P(A;;FA;;;SY)(A;;FR;;;BU)", "S-1-5-32-545"},
		{"inherited users read", "O:BAD:AI(A;;FA;;;SY)(A;ID;0x1200a9;;;BU)", "S-1-5-32-545"},
		{"owned by users", "O:BUD:P(A;;FA;;;SY)", "owned by S-1-5-32-545"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sd, err := windows.SecurityDescriptorFromString(tc.sddl)
			if err != nil {
				t.Fatal(err)
			}
			err = checkWindowsDACL("auth_key_file", `C:\ProgramData\ts-bridge\authkey`, sd)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("checkWindowsDACL() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "auth_key_file") {
				t.Errorf("checkWindowsDACL() = %v, want an error naming auth_key_file and %q", err, tc.wantErr)
			}
		})
	}
}
