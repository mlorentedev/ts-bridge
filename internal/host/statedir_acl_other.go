//go:build !windows && !linux

package host

// readAccessACL is nil off Linux: a macOS extended ACL is reachable only
// through acl_get_file in libc, not through a syscall x/sys/unix wraps, and the
// project builds without cgo. An ancestor's ACL is therefore not read there;
// docs/runbooks/host-serve.md says so.
var readAccessACL func(string) ([]byte, error)
