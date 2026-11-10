---
tags: [spec, tasks]
created: "2026-11-04"
id: "HOST-561-state-dir-ancestors"
type: spec-tasks
status: active
owner: manu
---

# Tasks - HOST-561-state-dir-ancestors

## Setup

- [x] Branch from master: `fix/issue-561-state-dir-ancestors`
- [x] `proposal.md` complete; forks decided (both functions walk; a root-owned link is followed to its target, changed after review on #596)

## Implementation

- [x] [AC1] [AC2] Failing table test for `checkUnixAncestor`, the pure per-ancestor rule:
  root 0755 dir ok; 1777 sticky refused; group-writable refused; user-owned refused; root-owned
  link ok; user-owned link refused; file refused
- [x] [AC1] [AC2] [AC3] Failing table test for `walkStateDirAncestors` with an injected `lstat`:
  `/var/lib/ts-bridge/serve` ok; macOS `/var` link ok (target walked); macOS `/tmp` link refused naming `/private/tmp`; `/tmp/x/serve` refused naming `/tmp`;
  user link refused naming it; absent ancestors skipped; the walk never consults the leaf
- [x] Implement both, wire into `CheckStateDir` and `EnsureStateDir` (before create)
- [x] [AC1] Test: `EnsureStateDir` creates nothing when the walk refuses
- [x] Stub the walk in the existing real-filesystem tests

## Closing

- [x] `go test -race ./...`, `golangci-lint run`, `gosec ./...`, `GOOS=windows go vet ./...`
- [x] Mutation: drop the group/other-write test → a case fails
- [x] `verification.md` filled; `docs/runbooks/host-serve.md` state_dir rule updated
