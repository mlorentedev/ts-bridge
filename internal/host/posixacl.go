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
	aclMask  = 0x10

	aclWrite = 0x02
)

// aclWriters returns the principals, other than root, that the access ACL
// raw lets write: a named user or group entry whose write bit survives the
// mask. The owner and other classes are st_mode's, already judged; a mask
// that removes a named entry's write leaves it ineffective, and widening the
// mask later shows in the group bits, which the mode check reads. A named
// entry with no mask to limit it counts as effective. A malformed value is an
// error: an ACL that cannot be read is not one that grants nothing.
func aclWriters(raw []byte) ([]string, error) {
	if len(raw) < aclHeaderSize || (len(raw)-aclHeaderSize)%aclEntrySize != 0 {
		return nil, fmt.Errorf("malformed POSIX ACL: %d bytes", len(raw))
	}
	if v := binary.LittleEndian.Uint32(raw); v != aclXattrVersion {
		return nil, fmt.Errorf("unsupported POSIX ACL version %d, want %d", v, aclXattrVersion)
	}
	mask, masked := uint16(0), false
	type entry struct {
		tag, perm uint16
		id        uint32
	}
	var named []entry
	for off := aclHeaderSize; off < len(raw); off += aclEntrySize {
		e := entry{
			binary.LittleEndian.Uint16(raw[off:]),
			binary.LittleEndian.Uint16(raw[off+2:]),
			binary.LittleEndian.Uint32(raw[off+4:]),
		}
		switch e.tag {
		case aclMask:
			mask, masked = e.perm, true
		case aclUser, aclGroup:
			named = append(named, e)
		}
	}
	var out []string
	for _, e := range named {
		perm := e.perm
		if masked {
			perm &= mask
		}
		if perm&aclWrite == 0 || e.id == 0 {
			continue
		}
		kind := "user"
		if e.tag == aclGroup {
			kind = "group"
		}
		out = append(out, fmt.Sprintf("%s %d", kind, e.id))
	}
	return out, nil
}
