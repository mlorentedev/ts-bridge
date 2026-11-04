---
id: "HOST-561-state-dir-ancestors"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-11-04"
issue: "mlorentedev/ts-bridge#561"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# HOST-561-state-dir-ancestors

## Why

On Unix, `host.CheckStateDir` and `EnsureStateDir` (HOST-008, #556) `Lstat` only the last element
of `host.mesh.state_dir`. Suppose an ancestor is writable by another local user, or is a symlink that
user planted. That user can then place the root-owned `0700` key store under a tree they control.
They cannot read the node key, but they can rename the directory away and break the node's identity
across restarts. The default `/var/lib/ts-bridge/serve` is not exposed. A `state_dir` under `/tmp` or
`/var/tmp` is. The Windows half already walks every ancestor (#561, raised in the review of #556).

## What

Both functions now walk every existing ancestor of the cleaned `state_dir` before anything else,
using `Lstat`:

- a directory must be owned by root and must not be writable by group or others. This includes a
  sticky world-writable directory such as `/tmp`;
- a symlink must be owned by root, and its target is then walked by the same rules, as the kernel
  resolves it: top-down, a `..` in a target taken from where the link points, at most 40 links over
  the whole walk. A `..` in the declared `state_dir` is refused, because `Clean` would drop it
  lexically. Root owning a link says nothing about who can write where it
  points: macOS's `/tmp` → `/private/tmp` is 1777. AC2's `/var/run` → `/run` and macOS's `/var` →
  `/private/var` still pass, because their targets are root-owned directories;
- anything else (a file, a device) is refused.

An ancestor that does not exist yet is skipped: `EnsureStateDir` creates it with mode `0700`. The
first failing ancestor is named in the error, which keeps the `state_dir <path>:` prefix.
`EnsureStateDir` walks before it creates anything.

## Out of scope

- The Windows half, which already walks its ancestors.
- `auth_key_file`'s ancestors (`CheckSecretFile`). The same question applies there, but it is a
  separate contract; a follow-up issue tracks it.

## Risks / open questions

- A root-owned link is followed rather than trusted. The draft trusted it, and review on #596
  showed that this let macOS `/tmp` pass. Following it refuses nothing AC2 keeps. Decided.
- A real-filesystem test cannot run the walk as non-root: every `t.TempDir()` sits under `/tmp`
  (Linux) or a user-owned `/var/folders/…` (macOS). Mitigation: the per-ancestor rule is a pure
  function over `fs.FileInfo` and a uid, and the walk takes an injected `lstat` and `readlink`. Existing
  real-filesystem tests stub the walk through a package seam.

## Acceptance criteria

- [ ] AC1: a `state_dir` under a world-writable ancestor, or under a symlink owned by another user,
  is refused. The error names the ancestor, and nothing is created.
- [ ] AC2: `/var/lib/ts-bridge/serve` and root-owned symlinked ancestors such as `/var/run` (and
  macOS's `/var`) are still accepted.
- [ ] AC3: `CheckStateDir` applies the same walk, so an existing `state_dir` under `/tmp` is refused
  too.

## References

- Issue #561; HOST-008 (#522, #556); `internal/host/statedir_windows.go` (the Windows ancestor walk)
