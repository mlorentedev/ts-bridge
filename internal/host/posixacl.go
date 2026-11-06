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

	aclUserObj  = 0x01
	aclUser     = 0x02
	aclGroupObj = 0x04
	aclGroup    = 0x08
	aclMask     = 0x10
	aclOther    = 0x20

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
	if err := aclHeaderValid(raw); err != nil {
		return nil, err
	}
	if err := aclShapeValid(raw); err != nil {
		return nil, err
	}
	var out []string
	for off := aclHeaderSize; off < len(raw); off += aclEntrySize {
		if w, ok := aclEntryWriter(raw[off : off+aclEntrySize]); ok {
			out = append(out, w)
		}
	}
	return out, nil
}

// aclHeaderValid refuses a value that is not a whole number of entries after
// the header, or that carries a version other than the one the kernel writes.
func aclHeaderValid(raw []byte) error {
	if len(raw) < aclHeaderSize || (len(raw)-aclHeaderSize)%aclEntrySize != 0 {
		return fmt.Errorf("malformed POSIX ACL: %d bytes", len(raw))
	}
	if v := binary.LittleEndian.Uint32(raw); v != aclXattrVersion {
		return fmt.Errorf("unsupported POSIX ACL version %d, want %d", v, aclXattrVersion)
	}
	return nil
}

// aclEntryWriter names the principal of one entry when it is a named user or
// group, other than root, with the write bit set.
func aclEntryWriter(entry []byte) (string, bool) {
	tag := binary.LittleEndian.Uint16(entry)
	perm := binary.LittleEndian.Uint16(entry[2:])
	id := binary.LittleEndian.Uint32(entry[4:])
	if (tag != aclUser && tag != aclGroup) || perm&aclWrite == 0 || id == 0 {
		return "", false
	}
	kind := "user"
	if tag == aclGroup {
		kind = "group"
	}
	return fmt.Sprintf("%s %d", kind, id), true
}

// aclShapeValid applies the kernel's posix_acl_valid shape: exactly one
// USER_OBJ, GROUP_OBJ and OTHER, and a MASK once a named entry exists. A value
// a conforming kernel never stores (a FUSE or network filesystem's) is refused
// rather than read as an ACL that names no writer.
func aclShapeValid(raw []byte) error {
	seen := map[uint16]int{}
	for off := aclHeaderSize; off < len(raw); off += aclEntrySize {
		seen[binary.LittleEndian.Uint16(raw[off:])]++
	}
	for _, tag := range []uint16{aclUserObj, aclGroupObj, aclOther} {
		if seen[tag] != 1 {
			return fmt.Errorf("malformed POSIX ACL: want one entry with tag %#x, found %d", tag, seen[tag])
		}
	}
	if seen[aclUser]+seen[aclGroup] > 0 && seen[aclMask] != 1 {
		return fmt.Errorf("malformed POSIX ACL: named entries without one mask entry")
	}
	return nil
}
