---
spec: "CFG-005-self-contained-connections"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "3c61bd2db1abe6b09523fc68934f7b691f74bea9"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-21"
---

## Adversarial review

**Scope**: CFG-005-self-contained-connections (PR #442, draft, head `3c61bd2`)
**Sources**: `specs/CFG-005-self-contained-connections/{proposal,tasks,verification,features}.md`
+ `review-request.json`; `git diff ecfa215...HEAD` (branch point to head, 26 files / 1503
insertions) and `git diff 43da717..HEAD` (post-CI commits); read in full:
`internal/profile/{store,lock,lock_unix,lock_windows,runtime_test,lock_test}.go`,
`cmd/cli/{connect,browser,init,profile_credentials,profile_runtime_test}.go`,
`internal/config/{merge,yaml}.go`, `internal/bootstrapssh/bootstrap.go`,
`docs/adr/adr-016-*.md`, `docs/runbooks/guide-profile-connections.md`.

### Evidence produced by this review (head `3c61bd2`, go1.27.1 windows/amd64)

| Command | Result |
|---|---|
| `go test ./internal/profile -run Runtime -count=1` (f1) | `ok  ts-bridge/internal/profile  1.695s` |
| `go test ./cmd/cli -run ProfileRuntime -count=1` (f2) | `ok  ts-bridge/cmd/cli  5.868s` |
| `go test ./internal/profile ./cmd/cli -run ProfileLock -count=1` (f3) | `ok` both packages |
| `go test ./internal/bootstrapssh -run 'TestStartReports(MissingOpenSSH\|EarlyExit\|UnlaunchableOpenSSH)' -count=1` (f4) | `ok  ts-bridge/internal/bootstrapssh  1.091s` |
| `go test ./... -count=1` | all 13 packages `ok`, 0 failures |
| `go build ./...` / `go vet ./...` | clean |
| `gh pr view 442` | head `3c61bd2`; `test`, `test-windows`, `smoke`, `lint`, `security`, 6× `build-matrix`, `hygiene`, GitGuardian all `SUCCESS`; pr-agent `review` = **SKIPPED** |
| `dotf pr triage-queue` | `[OK] no reviewer output is awaiting a disposition`, exit 0 |
| normalized contract digests recomputed locally | proposal `9871c0ea…`, tasks `2bad489e…`, features `765ac550…` — **equal to `review-request.json`**, so this verdict is fresh against the current contract |
| f5 (live Gitea probe) | **not run** — needs `TS_BRIDGE_PROBE_ORIGIN`/`TS_BRIDGE_PROBE_LOCAL` plus a powered-on upstream; operator-only |

### Spec and task alignment

- AC1 ↔ `TestProfileRuntimePreserved` (5 update paths incl. `Import`, `SetCredential`, `ClearCredential`); `Runtime` is `yaml:",inline"` with `omitempty`, so legacy stores load unchanged and `Import` carries `existing.Runtime` forward. Descriptor export is structurally impossible: `profile.Descriptor` has no runtime fields and the diff does not touch `descriptor.go`.
- AC2 ↔ `TestProfileRuntimeConnectPrecedence` (profile→YAML→env→flag, four cases), `TestProfileRuntimeImportPreservesExistingTargetAndCredential`, `TestProfileRuntimeImportRejectsUnsafeOrUnsupportedInput` (9 negative cases), `TestProfileRuntimeBrowserDoesNotInheritTCPTarget`. Precedence is real, not asserted: `Merge` runs `applyYAML` → `applyEnv` → `applyFlags`, and the new `local_addr` is fed through `applyYAML`, so it is overridden by `TS_LOCAL_ADDR` like every other field. `applyAutoInstance` only fills an empty `LocalAddr`, so a profile value survives auto-mode.
- AC3 ↔ `TestProfileLockLifecycle` (duplicate rejected, `../office` cannot escape the ownership dir, lock file persists), `TestProfileLockReleasedAfterProcessExit` (**real second process**, killed with `Process.Kill`, ownership recoverable), `TestProfileLockRejectsDuplicateBeforeRunner` (`runs==1 && logs==1` for both `connect` and `browser`), `TestProfileLockAllowsOtherProfile`. Locking is wired before `LoggerInit` in both runner commands.
- AC4 ↔ `TestStartReportsMissingOpenSSH`, `TestStartReportsEarlyExit`, `TestStartReportsUnlaunchableOpenSSH`; `startupError` names the resolved client and the error cause without touching terminals or the auth key.
- AC5: proposal box is **unticked**, `tasks.md` leaves the live task open, while `verification.md` records the probe satisfied on 2026-10-20. The live narrative is detailed and falsifiable (hashes, ports, NLA negotiation bytes, duplicate exit 1), but it is not reproducible from the repo and I did not re-run it.
- Discipline Gate: no new module or version was introduced (`golang.org/x/sys` moved indirect→direct, `v0.49.0` already in the graph); the one public-surface change (`init --config` in profile mode) replaces a previous hard rejection, so no silent behaviour flip.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | THEORETICAL | concurrency / durability | `init --profile` is the store's only writer but takes **no** ownership lock, and `save()` is a non-atomic `os.WriteFile`; a one-time configuration run during a live `connect --profile <same>` can race the store and an interrupted write leaves a truncated `profiles.yaml` (the connection's target and credential *reference* are then lost; the credential file itself survives) | `cmd/cli/init.go:202` → `profile.Store.Configure` → `save()` `os.WriteFile` (`internal/profile/store.go:234-248`); `lockCommandProfile` is wired only at `cmd/cli/connect.go:150` and `cmd/cli/browser.go:59`. Live acceptance in `verification.md` ran exactly this overlap and bytes were preserved — no failure observed | UNTESTED (no test exercises `init --profile` against a held lock, and none asserts store integrity after a partial write) | code + tests (temp-file+rename in `save`, or take `Lock` in `runInitProfile`); otherwise ticket with this root cause |
| Minor | REAL | docs vs records | Runbook says "Live Windows/Git Bash acceptance remains pending in the feature's verification record" and keeps `status: draft`, while `verification.md` records AC5 satisfied — two committed docs disagree about the feature's state | `docs/runbooks/guide-profile-connections.md` frontmatter + opening paragraph vs `verification.md` "Final acceptance (2026-10-20)" | none (documentation; the `hygiene` job checks frontmatter only) | docs (runbook — outside the contract set, fixable now) |
| Minor | REAL | verification record | `verification.md` records CI green only on `43da717` and states "Unix lock behavior has not been executed on this Windows host" / "Race detector: not run"; the actual head `3c61bd2` **is** green including Linux `-race` and native Windows, so the record both understates the evidence and omits the post-#444-merge head | `gh pr view 442 --json statusCheckRollup` on `3c61bd2` (all CI jobs SUCCESS, 2026-10-20T20:32-20:36Z) | n/a — evidence record; the unlock/race claim is covered by the `test` job running `TestProfileLockReleasedAfterProcessExit` | spec (`verification.md` — outside the contract set) |
| Minor | REAL | handoff-readiness | The contract artifacts still read "in progress" for a change whose evidence says otherwise: `proposal.md` AC5 `- [ ]` and `status: implementing`; `tasks.md` leaves the CI box, the live-AC5 box and six Closing boxes unticked; `features.json` has all five features `state: pending` / `evidence: ""` (correct only if a harness will ever run them — none is wired here) | `proposal.md` AC list; `tasks.md` Implementation/Closing; `features.json` | n/a — checkbox ticks and `state`/`evidence` are folded by `ContractDigests`, so ticking them does **not** stale this review | spec (tick + disposition at archive; no text change needed) |
| Minor | THEORETICAL | secrets | The "non-secret" guarantee is enforced by a **prefix** list only (`tskey-`, `hskey-`). A `control_url` such as `https://user:password@vpn.example/` or a `bootstrap_ssh` value carrying inline credentials is accepted and persisted; `ValidateControlPlane` checks only the scheme | `internal/profile/store.go` `Configure` loop over `secretPrefixes`; `internal/config/config.go:376-389` | UNTESTED (no negative case for URL userinfo) | code (reject `url.User != nil`) or declare it out of scope; otherwise ticket |
| Minor | REAL | usability / spec silence | A runtime field can never be **unset** through the supported path: `importProfileConfig` applies only non-empty values and `Configure` replaces `Runtime` wholesale from the preserved selection, so removing e.g. `bootstrap_ssh` means hand-editing `profiles.yaml`. The runbook documents "omitted … preserved" but not the absence of an unset path | `cmd/cli/profile_credentials.go` `importProfileConfig` (`if field.value != ""`) | UNTESTED (no test because no path exists) | docs (runbook) + follow-up ticket |
| Minor | REAL | doc precision | "A second `connect` or `browser` using the same profile fails before initializing logging" is true only when the profile is selected by `--profile`; the same connection expressed through `TS_*` env vars takes no lock and fails later, at the occupied listener (the case the proposal's #376 boundary acknowledges) | `lockCommandProfile` reads the `--profile` flag only; no `TS_PROFILE` env exists | `TestProfileLockRejectsDuplicateBeforeRunner` (proves the `--profile` half) | docs (runbook — one sentence of scope) |
| Minor | SPECULATIVE | maintainability | The caller-facing `Profile` now carries YAML tags including `credential`; nothing marshals it today except a test, so a future `yaml.Marshal(Profile)` would emit runtime settings **and** the credential reference into whatever artifact asked for it | `internal/profile/store.go` `Profile` struct; `internal/profile/runtime_test.go:48` is the only marshal site | `TestDescriptorRoundTrip` (today's guarantee is structural, not tested) | code (drop tags from `Profile`) — surface only, non-gating |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1–AC4 verified by named tests I re-ran green; AC5 rests on the operator's narrative, which I could not reproduce. |
| Verification       | B | Reproducible commands for four criteria, negative paths present; the CI/unix-lock record is stale and AC5 is not reviewer-reproducible (f5 needs live mesh inputs). |
| Scope              | B | Diff matches proposal; the extra material is the merged `#444`/BUG-031 work plus one documented, proposal-covered security correction of a sibling runbook. |
| Reliability        | B | Lock released by `defer` and by the OS on kill (test-proven), errors wrapped with remediation; open gaps are the non-atomic store write and the missing unset path. |
| Maintainability    | A | Small table-driven functions, low nesting, comments explain the precedence and the lock lifetime, `#nosec` justifications are narrow and specific. |
| Handoff-readiness  | B | ADR-016 + lesson-039 + afternoon runbook written in-session; contract artifacts and the runbook still read "in progress". |

### Verdict
**PASS WITH GAPS** — no Blocker; one Major that is THEORETICAL (never observed, no repro), so it is tracked rather than blocking; six Minors, one SPECULATIVE and non-gating. Rubric: all B or above, no C or D.

Per `harness/reviewer-pool.json` this verdict is signed by `nan/deepseek-v4-flash` (pool primary, the id `dotf spec review` recorded in `review-request.json`). Note that PR #442's own `pr-agent` `review` check is `SKIPPED` — that check is not a review, and this file is the only independent verdict on the change.

### Recommended next steps

Grouped by set, because the two have different rules. **Nothing here asks for a contract-set edit**: `proposal.md`, `tasks.md` and `features.json` are unchanged since this review and remain so until archive.

**Outside the contract set — safe to apply now (`verification.md` is exactly for this):**

1. Record this review in `verification.md` with a disposition line per finding above (applied / ticketed / declined with a reason), and add the current-head CI evidence (`3c61bd2`: `test`, `test-windows`, `smoke`, `lint`, `security`, `build-matrix`×6, `hygiene`, GitGuardian all green) beside the `43da717` entry. Restate the Unix-lock/race gap as closed by CI rather than "not run".
2. `docs/runbooks/guide-profile-connections.md`: drop the "acceptance remains pending" sentence, and scope the duplicate-ownership sentence to `--profile` selection (finding 7). Set `status: accepted` only if the operator agrees the contract is final.
3. Decide finding 4: fix in scope (temp-file+rename in `save`, or take `Lock` in `runInitProfile`) or ticket it with this root cause. Either disposition is acceptable for archive; silence is not.
4. Findings 5 and 6: file one ticket for URL-userinfo rejection in the "non-secret" guard and one for an explicit unset path, or decline them here with a reason.

**Contract set — at archive time only (ticking is folded out of the digest, so this does not stale the review):**

5. Tick `proposal.md` AC5, flip `status: archived` (the archive writes this itself), tick the two open `tasks.md` implementation boxes and the Closing boxes that are now true, and dispose each remaining Closing box explicitly rather than leaving it blank.
6. `features.json` `state`/`evidence` should be left to whatever runs the `verification` commands — I ran f1–f4 green this session, f5 remains operator-only. Do not hand-write `passing`.

`dotf spec archive CFG-005-self-contained-connections` is **advisable** in the current state once items 1–2 are applied; the archive gate will accept this verdict (fresh digests, pool-admitted signer). It is not advisable to archive before item 1, because a `review.md` nobody dispositioned in `verification.md` re-opens the same "records disagree" gap that finding 2 already flags.
