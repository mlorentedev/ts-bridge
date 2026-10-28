---
spec: "BUG-030-profile-target-source"
verdict: "FAIL"
reviewed_sha: "d1bc86809d65acce51ac0ca6ad9d2d1dc854fe00"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-23"
---

## Adversarial review

**Scope**: `BUG-030-profile-target-source` (issue mlorentedev/ts-bridge#417). Round 2, at
`d1bc868` (PR [#496](https://github.com/mlorentedev/ts-bridge/pull/496) head), after the
round-1 FAIL. The production change under review is `ca636e4` (#489, merged); `d1bc868`
carries the follow-up tests and verification evidence, and has **zero production diff**.
The launcher's declared range `3d1bb45..d1bc868` also contains unrelated host/doc PRs
(#485, #486, #488, #490); they are outside this spec and were not reviewed here.

**Sources**: `specs/BUG-030-profile-target-source/{proposal,tasks,verification}.md`,
`features.json`, `review-request.json`, the preserved `review-round1.md`,
`git diff 3d1bb45..d1bc868` restricted to the spec's files, `cmd/cli/connect.go`,
`cmd/cli/connect_target_source_test.go`, `internal/config/{config,merge}.go`,
`internal/logging/logging.go`, `cmd/cli/run.go`, `cmd/ts-bridge/main.go`,
`harness/reviewer-pool.json`, `gh pr view 489,496`.

**Independence**: reviewed on `nan/deepseek-v4-flash` (pool primary). No Anthropic model
signed; `proposal.md` declares no `risk: high`, so no second signature is required. Local
`golangci-lint` was **not** run (owner prohibition, `AGENTS.override.md`); the CI `lint`
result at the reviewed head is used instead.

**Contract state**: `dotf spec digest BUG-030-profile-target-source` returns the same three
digests the launcher recorded in `review-request.json`
(`features.json 8f6e577…`, `proposal.md 3b08b17…`, `tasks.md 78d893d…`), so the contract has
not moved since round 1 and this verdict is about the current contract state.

**Evidence produced in this session** (fresh, not quoted from `verification.md`):

```
go test ./cmd/cli -run 'TestConnectProfileTargetOverrideWarning|TestConnectTargetOverrideDefaultLogger|TestConnectTargetOverrideProductionLogger' -count=1 -v
  -> PASS, 26 table subtests + default-logger + 2 production-logger cases
go test ./cmd/cli ./internal/config/... ./internal/logging -count=1   -> ok (4 packages)
go vet ./...                                                          -> exit 0
git diff --check d1bc868^ d1bc868                                     -> exit 0
gh pr view 496 --json statusCheckRollup  -> test, test-windows, test-macos, lint, security,
  hygiene, smoke, 6 build-matrix legs, GitGuardian: SUCCESS; pr-agent/review: CANCELLED
```

**Mutation battery** (throwaway copy at `/tmp/b030`, no tracked file in this worktree was
modified; all mutations restored):

| Mutation | Expected discriminator | Observed |
|---|---|---|
| `defer` the warning so it runs after the runner | ordering | table `want exactly one warning before runner` fails in both formats; logger tests fail inside the runner |
| drop the `profileTarget == ""` guard | AC2 profile-without-target | `text/profile_without_target` and `json/…` fail on an unexpected warning |
| make `logging.New` console-only (no file) | AC3 JSON record | `TestConnectTargetOverrideProductionLogger/{text,json}` fail on `production logger did not initialize a log file` |
| let `TS_TARGET` outrank the flag in the label | AC1 source attribution | `…/flag_overrides_environment_and_YAML` fails in both formats (`source = "TS_TARGET", want "--target"`) |

The tests are not vacuous: each guard and each label the spec promises is independently
killable. The round-1 finding that a plain revert of the production file cannot reproduce
the recorded RED is now honestly restated in `verification.md`.

### Spec and task alignment

| Criterion | Verdict | Evidence |
|---|---|---|
| AC1 — precedence unchanged; changed profile target yields exactly one warning naming the winning source | **Met** | `applyConnectProfile` returns the target from the existing read; warning fires once after `Merge` and `lockCommandProfile`, naming `profile`/`source`/`target`. Flag, env, YAML and `.env` labels mutation-proven. |
| AC2 — no warning for profile-only, equal targets, no profile, SOCKS5, invalid config, profile-without-target | **Met for raw values, with one real edge (F3)** | Guard-first return; invalid config exits in `Merge`; `applyEnvProxyMode`/`applyFlagProxyMode` clear the target in SOCKS5 mode. Cases exist in both formats. |
| AC3 — text and JSON before the runner, visible with `--quiet`, no auth material, profile bytes unchanged | **Met** | `TestConnectTargetOverrideProductionLogger` drives the real `InitLogger` with temporary `HOME`/`LOCALAPPDATA`, asserts one text console line before a runner marker and one credential-free JSON file record under `--quiet`; the table compares profile-store bytes before/after. |

Board: #417 is `CLOSED`, #489 is `MERGED`, #496 is `OPEN` with this review as its
independent pass. Contract boxes that remain open are accurately open. `PR #496` has zero
reviews and a `CANCELLED` `pr-agent/review`; that is disclosed here rather than counted as
review coverage.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location |
|---|---|---|---|---|---|---|
| Major | REAL | governance / git record | The follow-up commit **`d1bc868`** — the head of PR #496, already pushed to `origin/test/profile-target-review-evidence` — carries `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`. This is the same non-negotiable standing-order violation round 1 recorded on the merged `ca636e4`, recurring in a **new, unmerged** commit. Unlike `ca636e4`, it is still cheap to remove: amend the message (or squash-merge #496 with a clean title/body) before it reaches `master`, where the owner's no-rewrite rule makes it permanent. Round 1 also asked for a bitácora ticket because no gate catches this; `gh issue list --search "Co-authored-by OR attribution OR Copilot" --state all` finds none, so the same default will append it again. | `git log -1 --format=%B d1bc868`; `git branch -a --contains d1bc868`; `gh pr view 496 --json headRefOid` | UNTESTED (no gate exists) | git record (amend/squash) — **not** the contract set; plus a ticket for a prevention gate |
| Minor | REAL | verification accuracy | `verification.md` ("Cross-platform/race/lint results for this follow-up must come from its own CI") and the #496 body ("This follow-up's CI results are pending") are stale: CI at `d1bc868` has completed — Linux `test` (`go test -v -race ./...`, `ci.yml:49`), `test-windows`, `test-macos`, `lint`, `security`, `hygiene`, `smoke`, six `build-matrix` legs and GitGuardian all `SUCCESS`. The honest statement now is "green at `d1bc868`; PR-Agent cancelled". This regressed only because CI finished after the commit. | `gh pr view 496 --json statusCheckRollup`; `.github/workflows/ci.yml:40-58` | the CI commands themselves are the evidence, and they pass | spec (`verification.md`, outside the contract set) |
| Minor | REAL | correctness edge (AC2) | Target equality is a raw string compare. Reproduced end to end in the throwaway copy: profile target `acemagic-office:45000` with `TS_TARGET=ACEMAGIC-OFFICE:45000` — the same DNS destination — emits `WARN connection target overrides selected profile source=TS_TARGET`. Round 1 graded this THEORETICAL; it is demonstrably REAL, still Minor (a diagnostic false positive, no behaviour change). Declined in round 1 on scope grounds (`proposal.md` specifies raw value difference); it needs a ticket or an explicit "raw comparison" note, not a silent carry-forward. | probe test output: `{"level":"WARN","msg":"connection target overrides selected profile","profile":"office","source":"TS_TARGET","target":"ACEMAGIC-OFFICE:45000"}`; `warnConnectTargetOverride` string compare | UNTESTED (no case covers case-differing equality) | code (case-insensitive host compare) or spec clarification — ticket, not a gate |
| Minor | THEORETICAL | maintainability / drift | The winning source is re-derived at the call site from `os.Getenv("TS_TARGET")` plus `flags.Target` rather than returned by `Merge`, duplicating the precedence that lives in `applyEnvProxyMode`/`applyFlagProxyMode`/`applyStringFields`. Every reachable combination I constructed agrees today (flag+env, env+YAML, SOCKS5 clearing, flag restores profile value, `.env`→env), so this is drift risk, not a defect. | `cmd/cli/connect.go:184-200` vs `internal/config/merge.go:330-380,474-484` | the four mutation cases above pin today's labels; no test fails if `Merge` changes precedence without this function | code (later; not a blocker) |

**Not findings (checked, then dismissed):** contract digests match the launcher's request, so
no contract drift; the round-1 `review.md` and `review-request.json` are preserved
byte-identically in `review-round1.md` / `review-request-round1.json` (verified by diff
against `d1bc868`); `gofmt -l` listing every file here is a CRLF checkout artifact
(`core.autocrlf=true`), not a formatting defect — the committed content is LF and CI
`hygiene` is green; no credential material is logged (profile name, source, `host:port`
only); the profile store is untouched on this path; `features.json` `state: pending` matches
the convention of every archived spec; `LoggerInit` is wired in `cmd/ts-bridge/main.go:34`,
so the `slog.Default()` fallback is the documented degraded path, not the shipped one; the
pre-existing `.env`-vs-process-env precedence/comment contradiction is disclosed and ticketed
under #399; `runConnect` never closing the logging instance is pre-existing (round-1 Q3).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness | B | Every AC reproduced; source labels mutation-proven; one real raw-comparison edge (F3) and the drift-prone re-derivation (F4). |
| Verification | B | Fresh commands, mutation battery and green CI all reproduce the claims; `verification.md` is stale about the follow-up's CI. |
| Scope | A | Production `+27/-5` in one file, tests `+258`, one runbook paragraph, one lesson, the spec; no dependency or contract change; #496 has zero production diff. |
| Reliability | A | Guard-first early return, no new error path, no warning on equal/SOCKS5/no-profile/invalid, profile bytes untouched, no secret in output. |
| Maintainability | B | Function well under 40 lines with low nesting; cost is the precedence duplicated at the call site. |
| Handoff-readiness | B | Lesson 045, runbook paragraph and PR `## Knowledge` present; the CI paragraph is stale and the attribution trailer is unresolved. |

Rubric alone (no C, no D) would read PASS. The findings axis escalates: one **REAL Major**.

### Verdict
**FAIL**

No functional defect was found in the warning itself: it is correct, narrow, credential-free,
`--quiet`-visible, ordered before the runner, and each promised property is independently
killable by mutation. The round-1 gaps it could close are closed: the real-logger/`--quiet`/
JSON-file path now has a test through `InitLogger`, the default-logger fallback is exercised
through the full command with its ordering asserted, and the profile-without-target guard has
a case. The FAIL is the durable record, not the behaviour: an AI-attribution trailer appended
to the **new** head commit, repeating the exact violation round 1 flagged, at the last point
where it is still amendable.

`dotf spec archive` is **not advisable** in this state: the contract digests are unchanged and
the folder is otherwise archive-ready, but a FAIL verdict is not a passing gate.

### Recommended next steps

Contract set (`proposal.md`, `tasks.md`, `features.json`) — **nothing to change**. A re-review
must not be invalidated by digest drift.

Non-contract, in order:

1. **Remove the trailer from `d1bc868`** (owner action, git record): `git commit --amend`
   without the `Co-authored-by` line, then force-push the review branch — or squash-merge
   #496 with a clean message. No code, test or contract change. This is the only blocker.
2. `verification.md`: replace the "CI must come from its own CI" sentence with the actual
   `d1bc868` result set (`test` on Linux with `-race`, `test-windows`, `test-macos`, `lint`,
   `security`, `hygiene`, `smoke`, six `build-matrix` legs, GitGuardian pass; `pr-agent/review`
   cancelled; local lint not run by policy). `verification.md` is outside the staleness set.
   Ticking `tasks.md`'s CI box is digest-neutral (SDD-042 ignores checkbox ticks) if the owner
   wants it consistent.
3. File the bitácora ticket round 1 asked for — the recurrence in `d1bc868` is the evidence
   that a Copilot-authored session will append the trailer again and nothing in this repo
   catches it.
4. Ticket (or explicitly decline in `verification.md`) the two inherited minors: F3 raw
   string equality and F4 source re-derivation. Neither blocks; both are now evidenced.

Re-review at the amended sha (or at a clean squash commit) should return **PASS WITH GAPS**
with F3/F4 tracked — the rubric has no C or D, and the remaining findings are minors.
