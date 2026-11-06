package host

import (
	"errors"

	"golang.org/x/sys/unix"
)

// readAccessACL returns the raw system.posix_acl_access xattr of path, or nil
// when it has none: no extended ACL, or a filesystem that keeps no ACLs. Any
// other failure is returned, so the walk refuses what it could not read.
func readAccessACL(path string) ([]byte, error) {
	for range 3 {
		size, err := unix.Getxattr(path, "system.posix_acl_access", nil)
		if noACL(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		buf := make([]byte, size)
		n, err := unix.Getxattr(path, "system.posix_acl_access", buf)
		if errors.Is(err, unix.ERANGE) {
			continue // the ACL grew between the two calls
		}
		if noACL(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return buf[:n], nil
	}
	return nil, errors.New("system.posix_acl_access kept changing while it was read")
}

// noACL reports whether err means path carries no access ACL: ENODATA when it
// has none, ENOTSUP when the filesystem keeps none, ENOENT when it is gone.
func noACL(err error) bool {
	return errors.Is(err, unix.ENODATA) || errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.ENOENT)
}
