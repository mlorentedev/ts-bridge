//go:build !windows && !linux

package host

import "testing"

// Off Linux no ancestor ACL is read: macOS extended ACLs are reachable only
// through libc's acl_get_file, and the build is pure Go. This pins the
// documented limitation so lifting it changes a test, not just the runbook.
func TestReadAccessACL_NotReadOffLinux(t *testing.T) {
	if readAccessACL != nil {
		t.Fatal("readAccessACL is set off Linux: update docs/runbooks/host-serve.md, which says ancestor ACLs are not read there")
	}
}
