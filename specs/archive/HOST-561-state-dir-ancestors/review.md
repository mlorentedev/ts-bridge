---
spec: "HOST-561-state-dir-ancestors"
verdict: "PASS"
reviewed_sha: "55cfd68ba8ef0f91d8ecfc12669895e1498e216c"
reviewer: "nan/mimo-v2.6-flash"
date: "2026-11-09"
---

## Adversarial review

**Scope**: HOST-561-state-dir-ancestors
**Sources**: `specs/HOST-561-state-dir-ancestors/{proposal,tasks,verification,features}.md|json`;
`git diff 5c4254291bc471dd3eb4112ef701bc202195571b...HEAD` (commits on the branch itself:
9ab3b55 #596 — the spec's PR, aba1a0c #612, 635945a #679, 8ad8da1 #676; the range also carries
~50 unrelated merged PRs per the launcher's framing — judged only where they interact with the
named contract); `review-request.json` contract digests re-verified fresh with
`dotf spec digest HOST-561-state-dir-ancestors` — all three match.

### Spec and task alignment

- **AC1** (world-writable / non-root / non-root-symlink ancestor refused, error names the
  ancestor, nothing created): verified. `TestCheckUnixAncestor` (9 subtests) and
  `TestWalkStateDirAncestors` (12 subtests incl. `/tmp/x/serve` → names `/tmp`, user's link,
  macOS `/tmp` → names `/private/tmp`) pass; `TestEnsureStateDir_RefusedAncestorCreatesNothing`
  ran on this machine's real filesystem (not skipped) and proves no `parent/` is created.
- **AC2** (default layout, absent ancestors, root-owned `/var/run` and macOS `/var` links still
  accepted): verified in `TestWalkStateDirAncestors` over the fake tree — the disclosed limitation
  (root-accept half cannot run non-root) matches the proposal's Risks section, which prescribes
  exactly this injected-`lstat` mitigation.
- **AC3** (`CheckStateDir` walks too): verified, `TestCheckStateDir_WalksAncestors` (real FS,
  passed here), plus `TestCheckStateDir_RefusesDotDot` and `TestCheckStateDir_Symlink`.
- **tasks.md**: every `[x]` has diff evidence — the two table tests, the walk tests with the
  leaf-never-read assertion, the no-create test, `skipAncestorWalk` stubs in the two pre-existing
  leaf tests, the runbook update (`docs/runbooks/host-serve.md` table now states the ancestor
  rule), and the closing commands (all re-run fresh below). "Branch from master:
  fix/issue-561-state-dir-ancestors" is unverifiable now (squash-merged; noted, not a defect).
- **Test-stub retention bar** (the change weakened two existing leaf tests by stubbing the walk):
  fields all present — stubbed tests: `TestCheckStateDir_Symlink`, `TestEnsureStateDir` (leaf
  behavior only); stronger proof shown: `TestEnsureStateDir_RefusedAncestorCreatesNothing` +
  `TestCheckStateDir_WalksAncestors` cover the walk on the real FS and were run; seam:
  `stateDirAncestors` is a package test var restored by `t.Cleanup`, unlocks no production
  removal; focused command run: `go test -race ./internal/host/`. Justified; no finding.
- **Kernel-semantics walk** (proposal's `..`-in-target, top-down, 40-link budget): code of
  `walkAncestors` matches the stated semantics; `..` pops from the resolved path (`cur`), a
  relative target resolves from the link's directory, one global link budget, fail-closed on
  `readlink`/ACL errors. Covered by `a link target's .. follows the link` and
  `TestWalkStateDirAncestors_LinearInLinks` (caps lstat calls at 40 for a 10-link nest).
- **Mutation probes (run, then reverted; tree clean)**: (a) `checkAncestor` group/other-write case
  disabled → 5 named subtests red; (b) user-owned-symlink uid check disabled → exactly 2 named
  subtests red (`TestCheckUnixAncestor/user-owned_link`, `TestWalkStateDirAncestors/a_user's_link`).
  Both match verification.md's claims (count note in Findings).
- **Scope**: the spec's own commits touch exactly proposal-named files + runbook. `auth_key_file`
  (CheckSecretFile) is declared out-of-scope with "a follow-up issue tracks it" (#595) and did
  land in-range under its own PR (#612) — documented in verification.md; the shared `walkAncestors`
  is parameterised by label, so the follow-up cannot drift the state_dir rule.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | THEORETICAL | path-resolution | `walkAncestors` assumes an absolute path (starts `cur` at `/`): given a relative `state_dir` it would check `"/"`-rooted components that do not exist, end the walk early and pass silently. Gated today: the only production caller (`cmd/cli/host_serve.go` `loadServePlan`) runs `LoadServeFile → File.validate → checkMeshPaths` first, which refuses a non-absolute `state_dir` before `EnsureStateDir` is reached. The walk itself does not enforce absoluteness — a latent coupling for future callers. | `internal/host/statedir_unix.go` (`components(filepath.Dir(path))`, `cur="/"`) vs `internal/host/mesh.go` `checkMeshPaths`; call order confirmed in `file.go`/`host_serve.go` | `TestLoadFile_MeshInvalid` case `"relative state_dir"` covers the gate; walk-on-relative-path itself UNTESTED | code (defensive `filepath.IsAbs` in `CheckStateDir`/`EnsureStateDir`) — follow-up ticket, outside contract set |
| Minor | REAL | verification evidence | verification.md claims the group/other-write mutant "fails 4 subtests"; the straightforward mutant (disabling the `perm&0o022` case) fails 5: `TestCheckUnixAncestor/{sticky_world-writable,group-writable,others-writable}` + `TestWalkStateDirAncestors/{macOS_/tmp_links_to_a_world-writable_dir,under_/tmp}`. Coverage is stronger than recorded; the count is off. | my mutation run this session (reverted; `git status` clean) | the mutation run itself | verification.md (outside contract set — disposition there) |
| Minor | THEORETICAL | maintainability | `checkUnixAncestor` is referenced only by its own test; production `ancestorAt` calls `checkAncestor` directly. The wrapper's doc comment carries the #561/ACL narrative while not being the production path, so a future rewiring of `ancestorAt` would not be caught by `TestCheckUnixAncestor` — mitigated because `TestWalkStateDirAncestors` walks the real `ancestorAt → checkAncestor` path with injected `lstat`/`readlink`. | `grep checkUnixAncestor(` → only `statedir_test.go` | `TestWalkStateDirAncestors` (covers the real path) | code (inline the alias, or route production through it) |
| Minor | SPECULATIVE | platform gap | On macOS an extended ACL granting write on an ancestor is not read (`readAccessACL` is nil, no cgo) — disclosed in `docs/runbooks/host-serve.md` and the code comment. Currently unreachable for `host serve`: `validateMesh` refuses `host.mesh` on macOS until #459, before `EnsureStateDir` runs. No acceptance criterion promises macOS ACL checking. | `statedir_acl_other.go`; runbook prose; `mesh.go` `capab.serve` gate | `statedir_acl_other_test.go` (build-tag presence only) | vault/docs: none needed; surface only |

No Blocker, no Major. Code-level checklist: no injection surface (pure path/xattr reads, no shell),
no secrets touched, permission validation *is* the feature and fails closed, walk is linear
(40-link budget, `TestWalkStateDirAncestors_LinearInLinks`), no unbounded loops, no dead code
(`deadcode`/`golangci-lint` clean); `walkAncestors` exceeds CC 10 under the repo's sanctioned
shrinking baseline (`//nolint:gocyclo // baseline #629`).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | All three ACs verified with named tests incl. negative/abuse paths (symlink leaf, `..`, loops, sticky `/tmp`); no observed defect. |
| Verification       | B | Every claim reproduced fresh (suite, lint, gosec, both mutations); one recorded mutant count off by one (5 vs 4). |
| Scope              | B | Spec's own commits match the proposal exactly; the range carries ~50 unrelated PRs (launcher-framed) and the declared out-of-scope `auth_key_file` follow-up landed in-range under its own issue — both documented. |
| Reliability        | A | Fail-closed on readlink/ACL/lstat errors, creates nothing on refusal (real-FS test), idempotent second run, never repairs an existing dir, `O_NOFOLLOW` leaf creation. |
| Maintainability    | B | Structured, commented WHY-code; CC within the repo's enforced baseline except one sanctioned nolint; test-only `checkUnixAncestor` alias quirk. |
| Handoff-readiness  | B | Proposal/tasks/verification/runbook all updated and promotion candidates answered `no: <reason>`; verification evidence checkboxes left unticked (cosmetic, pre-archive state). |

### Evidence (commands run this session, at reviewed_sha 55cfd68)

- `go test -race ./...` → exit 0, 17 packages ok, 0 failures.
- `go test ./internal/host -run 'TestCheckUnixAncestor\|TestWalkStateDirAncestors\|TestEnsureStateDir_RefusedAncestorCreatesNothing' -count=1` → ok (features.json f1 selector).
- `go test ./internal/host -run 'TestWalkStateDirAncestors'` (f2) and `-run 'TestCheckStateDir_WalksAncestors'` (f3) → ok; neither skipped.
- `bash scripts/check-features-selectors.sh` → exit 0, "50 go test -run selectors" OK.
- `golangci-lint run` (2.14.0, matches `mise.toml`) → 0 issues; `gofmt -l .` empty; `go vet ./...` and `GOOS=windows go vet ./...` → ok; `gosec ./...` → 0 issues.
- Mutation A and B (above) → red as claimed, both reverted; final `git status --porcelain` shows no modified tracked files.
- `dotf spec digest HOST-561-state-dir-ancestors` → matches `review-request.json` `contract_digests` exactly (freshness gate OK).

**UNVERIFIED**: `dotf spec archive` pre-flight not executed (running it would move the folder —
archiving is not this deliverable). Observed from `--help`: draft-tag scan (no
`[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` in proposal/tasks/verification), this review satisfies the
second pre-flight, and the third (promotion-candidate answers) is satisfied by inspection.
Coverage floor (`make cover-check`) and the e2e VM harness not run — CI/e2e-managed, outside this
spec's named verification.

### Verdict
PASS

### Recommended next steps
- Disposition the two Minor evidence/maintainability findings in `verification.md` (outside the
  contract set): correct the mutant count (4 → 5) or note the mutant form used, and either inline
  `checkUnixAncestor` or note why the alias stays.
- The relative-path hardening (defensive `filepath.IsAbs` inside `CheckStateDir`/`EnsureStateDir`)
  belongs in a follow-up ticket, not this contract: the gate is already enforced and tested at
  `checkMeshPaths`. Contract set stays closed — no `proposal.md`/`tasks.md`/`features.json` edits.
- `dotf spec archive HOST-561-state-dir-ancestors` is **advisable** in this state: verdict is
  passing, contract digests are fresh, no draft tags in the contract files, promotion candidates
  answered.
