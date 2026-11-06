package host

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

// readAccessACL reads what the kernel stores: a directory with a named-user
// entry set through the xattr returns those bytes, and one without returns nil.
func TestReadAccessACL(t *testing.T) {
	dir := t.TempDir()
	if raw, err := readAccessACL(dir); err != nil || raw != nil {
		t.Fatalf("readAccessACL(plain dir) = %v, %v, want nil, nil", raw, err)
	}
	acl := aclBytes(2, with(aclEnt{tagUser, 7, 1000}, aclEnt{tagMask, 7, noID})...)
	if err := unix.Setxattr(dir, "system.posix_acl_access", acl, 0); err != nil {
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EPERM) {
			t.Skipf("filesystem keeps no POSIX ACLs here: %v", err)
		}
		t.Fatal(err)
	}
	raw, err := readAccessACL(dir)
	if err != nil {
		t.Fatal(err)
	}
	writers, err := aclWriters(raw)
	if err != nil || len(writers) != 1 || writers[0] != "user 1000" {
		t.Fatalf("aclWriters(read ACL) = %v, %v, want [user 1000]", writers, err)
	}
	if raw, err := readAccessACL(dir + "/missing"); err != nil || raw != nil {
		t.Fatalf("readAccessACL(missing) = %v, %v, want nil, nil", raw, err)
	}
}
