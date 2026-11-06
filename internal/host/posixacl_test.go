//go:build !windows

package host

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"slices"
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

// with is base plus the named entries, in the order the kernel stores and
// accepts them (kernelValid): USER_OBJ, USER*, GROUP_OBJ, GROUP*, MASK, OTHER.
func with(extra ...aclEnt) []aclEnt {
	rank := map[uint16]int{tagUserObj: 0, tagUser: 1, tagGroupObj: 2, tagGroup: 3, tagMask: 4, tagOther: 5}
	all := append(append([]aclEnt{}, base...), extra...)
	slices.SortStableFunc(all, func(a, b aclEnt) int { return rank[a.tag] - rank[b.tag] })
	return all
}

// kernelValid mirrors posix_acl_valid (fs/posix_acl.c), which setxattr runs on
// system.posix_acl_access: USER_OBJ, USER*, GROUP_OBJ, GROUP*, MASK, OTHER in
// that order, and a MASK once a named entry exists. The parser reads any order,
// so a hand-built fixture has to be checked against the kernel's, or the test
// that hands it to the real xattr fails with EINVAL where the parser test stays
// green.
func kernelValid(entries []aclEnt) error {
	const (
		wantUser  = iota // USER_OBJ seen, USER* and GROUP_OBJ next
		wantGroup        // GROUP_OBJ seen, GROUP* and MASK next
		wantOther        // MASK seen, OTHER next
		done
	)
	state, named := 0, false
	for i, e := range entries {
		ok := false
		switch {
		case e.tag == tagUserObj && i == 0:
			ok = true
		case e.tag == tagUser && state == wantUser && i > 0:
			named, ok = true, true
		case e.tag == tagGroupObj && state == wantUser && i > 0:
			state, ok = wantGroup, true
		case e.tag == tagGroup && state == wantGroup:
			named, ok = true, true
		case e.tag == tagMask && state == wantGroup:
			state, ok = wantOther, true
		case e.tag == tagOther && (state == wantOther || state == wantGroup && !named):
			state, ok = done, true
		}
		if !ok {
			return fmt.Errorf("entry %d (tag %#x) is not valid at that place", i, e.tag)
		}
	}
	if state != done {
		return errors.New("ACL ends before OTHER")
	}
	return nil
}

// The fixtures with() builds are what the real xattr is handed, so the kernel
// must accept them.
func TestWithBuildsKernelValidACLs(t *testing.T) {
	tests := map[string][]aclEnt{
		"owner only":       with(),
		"named user":       with(aclEnt{tagUser, 7, 1000}, aclEnt{tagMask, 7, noID}),
		"named group":      with(aclEnt{tagGroup, 2, 1000}, aclEnt{tagMask, 7, noID}),
		"users and groups": with(aclEnt{tagUser, 6, 7}, aclEnt{tagGroup, 3, 9}, aclEnt{tagUser, 4, 8}, aclEnt{tagMask, 7, noID}),
	}
	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			if err := kernelValid(entries); err != nil {
				t.Fatalf("with() = %v: %v", entries, err)
			}
		})
	}
	if err := kernelValid(append(append([]aclEnt{}, base...), aclEnt{tagUser, 7, 1000}, aclEnt{tagMask, 7, noID})); err == nil {
		t.Fatal("kernelValid accepted USER after OTHER, which setxattr answers with EINVAL")
	}
	if err := kernelValid(append([]aclEnt{base[0], {tagUser, 7, 1000}}, base[1:]...)); err == nil {
		t.Fatal("kernelValid accepted a named entry with no MASK")
	}
}

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
		// posix_acl_valid's shape: USER_OBJ, GROUP_OBJ and OTHER, plus a MASK once a
		// named entry exists. A value that breaks it is not one this read understands.
		{"header only", aclBytes(2), nil, "malformed"},
		{"mask only", aclBytes(2, aclEnt{tagMask, 7, noID}), nil, "malformed"},
		{"no other entry", aclBytes(2, aclEnt{tagUserObj, 7, noID}, aclEnt{tagGroupObj, 5, noID}), nil, "malformed"},
		{"named entry without a mask", aclBytes(2, with(aclEnt{tagUser, 7, 1000})...), nil, "malformed"},
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
		acl: func(string) ([]byte, error) {
			t.Error("read the ACL of a directory root does not own")
			return nil, nil
		},
	}
	_, err := ancestorAt("state_dir", "/srv/state", "/srv", afs)
	if err == nil || !strings.Contains(err.Error(), "owned by uid 1000") {
		t.Fatalf("ancestorAt() = %v, want the owner refusal", err)
	}
}
