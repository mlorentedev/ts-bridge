//go:build !windows

package host

import (
	"encoding/binary"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

type aclEnt struct {
	tag, perm uint16
	id        uint32
}

// Entries as the kernel numbers them: tag, then rwx as 4/2/1.
const (
	tagUserObj  = 0x01
	tagUser     = 0x02
	tagGroupObj = 0x04
	tagGroup    = 0x08
	tagMask     = 0x10
	tagOther    = 0x20
	noID        = 0xffffffff
)

// aclBytes builds a system.posix_acl_access value by hand: the version header,
// then one 8-byte entry each, little-endian.
func aclBytes(version uint32, entries ...aclEnt) []byte {
	b := binary.LittleEndian.AppendUint32(nil, version)
	for _, e := range entries {
		b = binary.LittleEndian.AppendUint16(b, e.tag)
		b = binary.LittleEndian.AppendUint16(b, e.perm)
		b = binary.LittleEndian.AppendUint32(b, e.id)
	}
	return b
}

// base is the three entries an ACL with no named principal holds.
var base = []aclEnt{{tagUserObj, 7, noID}, {tagGroupObj, 5, noID}, {tagOther, 5, noID}}

func with(extra ...aclEnt) []aclEnt { return append(append([]aclEnt{}, base...), extra...) }

func TestAclWriters(t *testing.T) {
	tests := []struct {
		name    string
		raw     []byte
		want    []string
		wantErr string
	}{
		{"owner only", aclBytes(2, base...), nil, ""},
		{"named user with write", aclBytes(2, with(aclEnt{tagUser, 7, 1000}, aclEnt{tagMask, 7, noID})...), []string{"user 1000"}, ""},
		{"named user read only", aclBytes(2, with(aclEnt{tagUser, 5, 1000}, aclEnt{tagMask, 7, noID})...), nil, ""},
		// Ineffective today, one chmod g+w from effective: refused all the same.
		{"named user write masked out", aclBytes(2, with(aclEnt{tagUser, 7, 1000}, aclEnt{tagMask, 5, noID})...), []string{"user 1000"}, ""},
		{"named group with write", aclBytes(2, with(aclEnt{tagGroup, 2, 1000}, aclEnt{tagMask, 7, noID})...), []string{"group 1000"}, ""},
		{"named root user with write", aclBytes(2, with(aclEnt{tagUser, 7, 0}, aclEnt{tagMask, 7, noID})...), nil, ""},
		{"named root group with write", aclBytes(2, with(aclEnt{tagGroup, 7, 0}, aclEnt{tagMask, 7, noID})...), nil, ""},
		{"every writer is named", aclBytes(2, with(aclEnt{tagUser, 6, 7}, aclEnt{tagGroup, 3, 9}, aclEnt{tagUser, 4, 8}, aclEnt{tagMask, 7, noID})...), []string{"user 7", "group 9"}, ""},
		{"empty", nil, nil, "malformed"},
		{"header only", aclBytes(2), nil, ""},
		{"truncated entry", aclBytes(2, base...)[:10], nil, "malformed"},
		{"version 1", aclBytes(1, base...), nil, "version 1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := aclWriters(tc.raw)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("aclWriters() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("aclWriters() = %v, %v, want %v", got, err, tc.want)
			}
		})
	}
}

// The walk refuses the ancestor, names it and the principal, and leaves what it
// did not need to read alone.
func TestAncestorAt_ACL(t *testing.T) {
	dir := fakeInfo{fs.ModeDir | 0o755}
	link := fakeInfo{fs.ModeSymlink | 0o777}
	writeACL := aclBytes(2, with(aclEnt{tagUser, 7, 1000}, aclEnt{tagMask, 7, noID})...)
	tests := []struct {
		name    string
		info    fakeInfo
		acl     func(string) ([]byte, error)
		wantErr string
	}{
		{"named user write refused", dir, func(string) ([]byte, error) { return writeACL, nil }, "ancestor /srv: its ACL grants write to user 1000"},
		{"no ACL as today", dir, func(string) ([]byte, error) { return nil, nil }, ""},
		{"masked out write refused", dir, func(string) ([]byte, error) {
			return aclBytes(2, with(aclEnt{tagUser, 7, 1000}, aclEnt{tagMask, 5, noID})...), nil
		}, "ancestor /srv: its ACL grants write to user 1000"},
		{"unreadable ACL refused", dir, func(string) ([]byte, error) { return nil, fs.ErrPermission }, "ancestor /srv: read its ACL"},
		{"malformed ACL refused", dir, func(string) ([]byte, error) { return []byte{1, 2, 3}, nil }, "ancestor /srv: malformed POSIX ACL"},
		{"ACLs not read", dir, nil, ""},
		{"a link is not read", link, func(string) ([]byte, error) { t.Error("read the ACL of a link"); return writeACL, nil }, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			afs := ancestorFS{
				lstat: func(string) (fs.FileInfo, uint32, error) { return tc.info, 0, nil },
				acl:   tc.acl,
			}
			_, err := ancestorAt("state_dir", "/srv/state", "/srv", afs)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ancestorAt() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ancestorAt() = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

// A directory root does not own fails on its owner, and its ACL is not read.
func TestAncestorAt_ACLNotReadForForeignOwner(t *testing.T) {
	afs := ancestorFS{
		lstat: func(string) (fs.FileInfo, uint32, error) { return fakeInfo{fs.ModeDir | 0o755}, 1000, nil },
		acl:   func(string) ([]byte, error) { t.Error("read the ACL of a directory root does not own"); return nil, nil },
	}
	_, err := ancestorAt("state_dir", "/srv/state", "/srv", afs)
	if err == nil || !strings.Contains(err.Error(), "owned by uid 1000") {
		t.Fatalf("ancestorAt() = %v, want the owner refusal", err)
	}
}
