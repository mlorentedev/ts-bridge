---
tags: [spec, verification]
created: "2026-11-04"
id: "HOST-561-state-dir-ancestors"
type: spec-verification
status: active
owner: manu
---

# Verification - HOST-561-state-dir-ancestors

## Evidence

- [ ] AC1 → three tests:
  - `TestCheckUnixAncestor` covers the rule. A sticky 1777 directory, a group-writable directory,
    an others-writable directory, a user-owned directory, a user-owned link and a file are each
    refused, and every error names `state_dir` and the ancestor.
  - `TestWalkStateDirAncestors` covers the walk: `/tmp/x/serve` is refused naming `/tmp`; a user's
    link and a user's directory deep down are refused, each naming itself; the leaf is never read.
  - `TestEnsureStateDir_RefusedAncestorCreatesNothing` uses the real filesystem. The `t.TempDir`
    tree is refused on Linux (under `/tmp`) and on macOS (user-owned), and the missing parent is
    never created.
- [ ] AC2 → `TestWalkStateDirAncestors` accepts four layouts: `/var/lib/ts-bridge/serve`, the same
  with absent ancestors, macOS's root-owned `/var` link, and a root-owned `/var/run` link.
- [ ] AC3 → `TestCheckStateDir_WalksAncestors` (real filesystem): `CheckStateDir` on an existing
  `0700` `t.TempDir` is refused for an ancestor.
- Mutants:
  - dropping the group/other-write test fails 4 subtests;
  - accepting a user-owned symlink fails 2.

## Test status

- `go test -race ./...` exit 0. `golangci-lint run ./...` reports 0 issues. `gosec ./...` exit 0.
  `GOOS=windows go vet ./...` exit 0.
- Not run as root on a real host. The root-accept half of AC2 rests on the fake-tree walk, because
  a non-root test cannot build a root-owned tree.

## Decisions made during implementation

- A root-owned link is followed, not trusted (review on #596). macOS `/tmp` is a root-owned link to
  a 1777 `/private/tmp`, so trusting the link let a `state_dir` under `/tmp` pass. The walk now
  checks the target and its ancestors, with at most 40 hops. Mutant: without the follow,
  `TestWalkStateDirAncestors` fails 4 cases (macOS `/tmp`, a link into a user's dir, a loop, a
  target's `..`).
- The walk resolves link targets as the kernel does (second review on #596). Before, a target was
  `Clean`ed lexically and each link re-walked its target's whole parent chain:
  - `/srv/link -> /srv/alias/../store` with `/srv/alias -> /tmp/nested` was checked at
    `/srv/store`, which the kernel never visits. On the old walk the new case
    `a link target's .. follows the link` returns nil; the new walk refuses `/tmp/store`.
  - Ten nested `/srv/l -> .` links cost the old walk 3071 `lstat` calls;
    `TestWalkStateDirAncestors_LinearInLinks` caps them at 40. One link budget now covers the whole
    walk, so only layouts the kernel itself refuses (ELOOP) are refused for links.
  - A `..` in `state_dir` is refused (`TestCheckStateDir_RefusesDotDot`); before, `/srv/link/../serve`
    was cleaned to `/srv/serve` and the link never walked.

- `CheckStateDir` walks too (AC3), not only `EnsureStateDir`. Otherwise an existing directory under
  `/tmp` would pass every later start.
- The ancestors are checked before the leaf. That makes the walk testable as non-root through the
  real filesystem. The two existing leaf tests on `t.TempDir` stub the walk (`skipAncestorWalk`).
- `auth_key_file` has the same gap. It is a separate contract, filed as #595.

## Promotion candidates

- [ ] Lesson for the repo's `docs/lessons/`? no: applies the Windows half's existing ancestor rule to Unix; nothing new learned
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: narrows an existing HOST-008 check, no new decision
- [ ] New pattern candidate for `00_meta/patterns/`? no: one occurrence

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/HOST-561-state-dir-ancestors/` -> `specs/archive/HOST-561-state-dir-ancestors/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [x] Promotions above executed (if any)

## Review disposition

`review.md` (nan/mimo-v2.6-flash) returned PASS with four Minors:

- Relative `state_dir` ends the walk early: declined. The only production caller resolves `state_dir` to an absolute path before the walk, so a relative path never reaches `walkAncestors`.
- Mutant count: corrected. Disabling the `perm&0o022` case fails 5 subtests, not 4 (`TestCheckUnixAncestor/{sticky_world-writable,group-writable,others-writable}` and two `TestWalkStateDirAncestors` rows).
- `checkUnixAncestor` is called only by its own test: declined. `TestWalkStateDirAncestors` drives the production path through `ancestorAt`, so a rewiring there is caught.
- macOS ACLs not read: declined. It is disclosed in `docs/runbooks/host-serve.md` and unreachable, because `validateMesh` refuses `host.mesh` on macOS until #459.
