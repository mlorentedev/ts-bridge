//go:build !windows

package host

import (
	"encoding/binary"
	"fmt"
)

// A Linux access ACL as the kernel stores it in the system.posix_acl_access
// xattr (struct posix_acl_xattr_header, then posix_acl_xattr_entry values):
// a little-endian u32 version, then {tag u16, perm u16, id u32} entries.
const (
	aclXattrVersion = 2
	aclHeaderSize   = 4
	aclEntrySize    = 8

	aclUser  = 0x02
	aclGroup = 0x08

	aclWrite = 0x02
)

// aclWriters returns the principals, other than root, that the access ACL raw
// names with write: a user or group entry whose write bit is set. The mask is
// deliberately not applied. A mask that hides an entry's write today is one
// chmod g+w from exposing it, and st_mode reads that mask as the group bits,
// so an entry the mask has not removed is already visible there; what only
// this read can see is the entry itself, and naming the principal. A malformed
// value is an error: an ACL that cannot be read is not one that grants nothing.
func aclWriters(raw []byte) ([]string, error) {
	if len(raw) < aclHeaderSize || (len(raw)-aclHeaderSize)%aclEntrySize != 0 {
		return nil, fmt.Errorf("malformed POSIX ACL: %d bytes", len(raw))
	}
	if v := binary.LittleEndian.Uint32(raw); v != aclXattrVersion {
		return nil, fmt.Errorf("unsupported POSIX ACL version %d, want %d", v, aclXattrVersion)
	}
	var out []string
	for off := aclHeaderSize; off < len(raw); off += aclEntrySize {
		tag := binary.LittleEndian.Uint16(raw[off:])
		perm := binary.LittleEndian.Uint16(raw[off+2:])
		id := binary.LittleEndian.Uint32(raw[off+4:])
		if (tag != aclUser && tag != aclGroup) || perm&aclWrite == 0 || id == 0 {
			continue
		}
		kind := "user"
		if tag == aclGroup {
			kind = "group"
		}
		out = append(out, fmt.Sprintf("%s %d", kind, id))
	}
	return out, nil
}
