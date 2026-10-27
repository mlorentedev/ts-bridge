---
spec: "BUG-030-profile-target-source"
verdict: "FAIL"
reviewed_sha: "1fe82ad1832f86f4ef6611edb1c8485a5328a03d"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-23"
---

## Adversarial review

**Scope**: `BUG-030-profile-target-source` (issue mlorentedev/ts-bridge#417, PR #489).
Diff `3d1bb45` (base, declared by the launcher in `review-request.json`) .. `1fe82ad`,
now squashed on `origin/master` as `ca636e4`.

**Sources**: `specs/BUG-030-profile-target-source/{proposal,tasks,verification}.md`,
`features.json`, `git diff 3d1bb45..1fe82ad`, `gh pr view 489`, `harness/reviewer-pool.json`,
`cmd/cli/connect.go`, `cmd/cli/connect_target_source_test.go`,
`internal/config/{config,merge}.go`, `internal/logging/logging.go`, `cmd/cli/run.go`.

**Independence**: reviewed on `nan/deepseek-v4-flash` (pool primary). No Anthropic model
signed, no second signature needed (`proposal.md` declares no `risk: high`). Local
`golangci-lint` was **not** run (owner prohibition, `AGENTS.override.md`); the CI `lint`
result at `1fe82ad` is used instead.

**Evidence produced in this session** (commands run fresh, not quoted from `verification.md`):

```
go test ./cmd/cli -run 'TestConnectProfileTargetOverrideWarning|TestConnectTargetOverrideDefaultLogger' -count=1   -> ok 8.09s
go test ./cmd/cli ./internal/config/... -count=1                                                                  -> ok (3 packages)
go vet ./...                                                                                                     -> exit 0
gofmt -l cmd/cli/connect.go cmd/cli/connect_target_source_test.go                                                -> empty
git diff --check 3d1bb45..1fe82ad                                                                                -> exit 0
```

Two independent re-derivations of the RED claim, in a throwaway copy of the tree (no
tracked file in this worktree was modified):

1. **Revert** `cmd/cli/connect.go` to the base revision → `undefined: warnConnectTargetOverride`
   (build failure). The recorded RED ("failed on the ten expected warning cases") is **not**
   reproducible by reverting alone; it required a no-op stub. The claim itself survives, its
   stated shape does not.
2. **Mutation** of the attribution: suppressing `source=--target` whenever `TS_TARGET` is set
   → `text/flag_overrides_environment_and_YAML` and `json/flag_overrides_environment_and_YAML`
   both fail, matching `verification.md`. The test genuinely discriminates the source label.

**Production-path probe** (real `cli.InitLogger`, `LOCALAPPDATA`/`HOME` redirected to a temp
dir, synthetic profile + key, `--profile office`, `TS_TARGET=env-host:443`):

| run | console stdout | log file |
|---|---|---|
| `--log-format text --quiet` | `level=WARN msg="connection target overrides selected profile" profile=office source=TS_TARGET target=env-host:443` | JSON record |
| `--log-format json --quiet` | **text**, same WARN record | JSON record |
| `--log-format text` | same WARN record | JSON record |

So AC3's `--quiet` visibility and the JSON record hold **for the writer that ships** — and the
console stays text under `--log-format json` (pre-existing `logging.New`: console text / file
JSON). That last fact is what makes finding F4 below a fidelity gap rather than a defect.

### Spec and task alignment

| Criterion | Verdict | Evidence |
|---|---|---|
| AC1 precedence unchanged + exactly one warning naming the winning source | **Met** | `applyConnectProfile` keeps its original ordering (`yamlCfg` fill → `applyProfileRuntime` → credential); warning fired once, after `Merge`, `cfg.Quiet`, `lockCommandProfile` and `LoggerInit`, before `Runner`. Mutation test proves the `--target` / `TS_TARGET` / `YAML` label is discriminating. |
| AC2 no warning for profile-only, equal targets, no profile, SOCKS5, invalid config | **Met** | Guards `profileName == "" || profileTarget == "" || target == "" || target == profileTarget`; invalid config returns from `Merge` before the call; SOCKS5 clears `cfg.Target` (`internal/config/merge.go` `applyEnvProxyMode`/`applyFlagProxyMode`). Table covers each case in both formats. |
| AC3 text/JSON before the runner, visible with `--quiet`, no auth material, profile bytes untouched | **Met in behaviour**; evidence fidelity is F4 | Probe above; `assertTargetOverrideWarning` asserts no `tskey-auth-`; `before`/`after` profile bytes compared per case. |

Board: issue #417 is `CLOSED` and PR #489 is `MERGED`; the spec folder is still active and
unarchived, which is the correct state until this review exists. Board and spec therefore agree.

Task boxes: 4 of 6 Closing/Implementation boxes remain open and are **accurately** open
(`Open an atomic PR … and inspect all reviewer output`, `Obtain an independent review`,
`Verify lint and Linux race tests through CI`) — except that CI has since run and passed, so
that last box's state is now stale (F3). Contract files (`proposal.md`, `tasks.md`,
`features.json`) are byte-identical to `origin/master`, so nothing here is drift.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location |
|---|---|---|---|---|---|---|
| Major | REAL | governance / git record | The merged commit carries `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` on `origin/master`. The standing order is explicit and non-negotiable: no AI attribution and no `Co-Authored-By` trailer referencing an AI agent, precisely because agent CLIs append it by default. It can no longer be fixed by amending the PR — it is in main history. | `git log -1 --format=%B ca636e4` contains the trailer; the trailer is also in local commit `1fe82ad`. | UNTESTED (no gate detects it) | code (owner decision: rewrite `master`, or accept + record in `verification.md`) **and** a bitácora ticket to gate it, since a Copilot-authored session will repeat it |
| Major | REAL | Definition of Done §4 (Review) | PR #489 **merged with no review at all**, and the only automated review run ended `CANCELLED`. `reviews` is `[]`; the `pr-agent / review` check is `CANCELLED` at `2026-10-22T20:29:17Z`; the author's own `## Review triage` comment six minutes earlier said "The PR remains open; no merge, archive or release was performed" and left PR-Agent "pending". Repo has **no** `.github/merge-grant.yml`, so the standing merge grant does not apply. Proceeding unreviewed is permitted only if disclosed — it was not. | `gh pr view 489 --json state,mergedAt,reviews,statusCheckRollup`; `ls .github/` | UNTESTED (process) | spec (`verification.md`, outside the contract set) + a PR comment; owner confirms who authorized the merge |
| Minor | REAL | verification accuracy | `verification.md` still says "Linux race tests and all-platform CI remain pending PR checks". They ran and passed at `1fe82ad`: `test` (Linux, `go test -v -race -coverprofile=… ./...`), `test-windows`, `test-macos`, `lint` (golangci-lint), `security`, `hygiene`, `smoke`, six `build-matrix` legs, GitGuardian — all `SUCCESS`. Only `pr-agent / review` is `CANCELLED`. | `gh pr view 489 --json statusCheckRollup`; `.github/workflows/ci.yml:50,115,186` | named commands exist; the *statement* is untested as written | spec (`verification.md`) |
| Minor | REAL | test fidelity (AC3 JSON) | The `json` table branch installs its own `slog.NewJSONHandler` on the console buffer, so it proves the *record encoding*, not the shipped writer: production `connect` never writes JSON to the console — JSON goes to the rotating log file. The same stub is what makes the `--quiet` case pass, so neither `--quiet` nor "JSON to the log file" is covered by a test of the real `InitLogger` path. Behaviour is nonetheless correct (probe, and the record is `level=WARN` against a `LevelWarn` console threshold). | `cmd/cli/connect_target_source_test.go:70-76`; `internal/logging/logging.go` `New`; probe output above | `TestConnectProfileTargetOverrideWarning` (proves the record, not the writer) | tests (one case through `InitLogger` + a log-file assertion) |
| Minor | THEORETICAL | correctness edge | Target equality is a raw string compare and the target is never normalised (`parseTarget` returns the env string verbatim; `Merge` trims only `ControlURL`). `TS_TARGET=MESH-Host:443` against a profile `mesh-host:443` is the same destination but yields an override warning — a false positive against AC2's "equal effective targets". | `internal/config/config.go` `parseTarget`; `warnConnectTargetOverride` | UNTESTED | code |
| Minor | THEORETICAL | maintainability / drift | The winning source is *re-derived* at the call site from `os.Getenv("TS_TARGET")` + `flags.Target` instead of being returned from `Merge`, duplicating the precedence that lives in `applyEnvProxyMode`/`applyFlagProxyMode`. Every reachable branch agrees today (env-vs-YAML, SOCKS clearing, flag short-circuit all checked), but a future precedence change silently mis-labels the warning and no test would notice. | `cmd/cli/connect.go:191-200` vs `internal/config/merge.go:365-390,470-485` | UNTESTED | code (later; not a blocker) |
| Minor | THEORETICAL | ordering coverage | "One warning **before** the runner" is proven only on the command path with a stubbed `LoggerInit`. With `LoggerInit == nil` the fallback is exercised by calling `warnConnectTargetOverride` directly, so its position relative to `Runner` is not asserted; the `profileTarget == ""` guard (selected profile without a target) has no case at all. | `TestConnectTargetOverrideDefaultLogger`; table fixture always sets a profile target | `TestConnectTargetOverrideDefaultLogger` (partial), `profileTarget == ""` UNTESTED | tests |

Not findings (checked, then dismissed): warning content carries no credential material
(profile name, source, `host:port` only); profile store is read-only on this path; the
untracked `review-transcript.jsonl` is git-ignored on purpose (`.gitignore:73`); the
`.env`-beats-process-env comment contradiction is pre-existing, disclosed, and ticketed
under #399; the `features.json` `state: pending` / empty `evidence` convention matches every
archived spec in this repo.

### Questions / assumptions (no action taken)

1. **Who authorized the merge?** The merge commit author and the triage comment are the same
   owner account; the comment says no merge was performed, then it merged. Either the owner
   merged deliberately (then the comment needs correcting) or it was not a deliberate merge.
   Only the owner can answer; F2 turns into "disclosed" once answered.
2. **Console stream choice.** The warning goes to `stdout` through the combined logger, not
   `stderr` (the older `--auth-key` warning uses `stderr`). That matches the existing
   `WARN` console lines and the `READY`/`ERROR` lifecycle lines, so it is consistent — surfaced
   only because a machine reading `READY` now has one more possible preceding line.
3. Pre-existing, not this diff: `runConnect` never closes the logging instance, so the log file
   handle stays open for the process lifetime (observed: the temp log file could not be removed
   while the probe process was alive). Out of scope; worth a ticket, not a gate.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness | B | Every AC reproduced independently and the source label is mutation-proven; two untested edges (case/whitespace equality, empty-profile-target guard). |
| Verification | B | Commands reproduce here, but the JSON/`--quiet` proof runs against a stub rather than the shipped writer, the recorded RED is a build failure when reverted, and the CI statement is stale. |
| Scope | A | Five code/doc files plus the spec, exactly the proposal; no creep, no dependency change, host code untouched. |
| Reliability | A | Guard-first early return, no new error path, no warning on invalid/SOCKS5/equal/no-profile, profile bytes untouched, no secret in output. |
| Maintainability | B | Function well under 40 lines and no nesting; the cost is precedence re-derived at the call site instead of returned from `Merge`. |
| Handoff-readiness | B | Lesson 045, runbook paragraph and PR `## Knowledge` all present; `verification.md` is stale about CI and silent about the unreviewed merge. |

Rubric alone (no C, no D) would read PASS. The findings axis escalates: two **REAL Majors**
force FAIL.

### Verdict
**FAIL**

No functional defect was found: the warning is correct, narrow, credential-free and
mutation-proven, and it ships the documented `--quiet`/JSON behaviour. The FAIL comes from the
durable record — an AI-attribution trailer on `master`, and a merge that landed with zero
review while its own triage comment still listed the reviewer as pending. Neither is a code
change, and neither needs a contract edit.

### Recommended next steps

Contract set (`proposal.md`, `tasks.md`, `features.json`) — **nothing to change**. That is
deliberate: the re-review after these dispositions will not be invalidated by digest drift.

Non-contract (fixable now, no re-review cost beyond the verdict itself):

1. `verification.md`: replace the pending-CI paragraph with the actual `1fe82ad` conclusions
   (`test` on Linux with `-race`, `test-windows`, `test-macos`, `lint`, `security`, `hygiene`,
   `smoke`, six `build-matrix` legs, GitGuardian — all pass; `pr-agent / review` cancelled;
   local lint not run by policy).
2. `verification.md` Findings-disposition table, once the owner answers Q1: the Copilot trailer
   (rewrite or accept-and-record) and the merge-without-review disclosure. Repo rule: unreviewed
   is allowed, silent is not.
3. One test through the real `InitLogger` (temp `LOCALAPPDATA`) asserting the JSON record lands
   in the log file and the console line survives `--quiet`; plus table cases for a profile with
   no target. Tests only.
4. Bitácora ticket for the attribution trailer: a Copilot session will append it again — the
   gate that should have caught it does not exist in this repo.
5. Owner decision on `master`: rewrite `ca636e4` (force-push, coordinated) or record the
   deviation and let the ticket carry prevention. Reviewer does not choose.

Re-review after 1–3 (and the Q1 answer) should return **PASS WITH GAPS** or **PASS** at the same
`1fe82ad` contract state.
