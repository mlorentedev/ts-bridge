---
tags: [spec, verification, templates]
created: "2026-10-22"
---

# Verification - BUG-030-profile-target-source

## Evidence

- [x] AC1: `TestConnectProfileTargetOverrideWarning` checks YAML, environment, `.env`, flags and competing lower-priority targets, preserving the runner's target.
- [x] AC2: The same table checks profile-only use, equal targets, a flag restoring the profile target, no profile, SOCKS5 mode and invalid target validation.
- [x] AC3: The table runs in text and JSON modes, checks exactly one warning before the runner, `--quiet`, absence of synthetic key material and unchanged profile-store bytes. `TestConnectTargetOverrideDefaultLogger` covers an absent initialization hook.

## Test status

- Baseline: `go test ./cmd/cli ./internal/config` passed before changes.
- Historical RED: `go test ./cmd/cli -run TestConnectProfileTargetOverrideWarning -count=1` was recorded as failing on the ten expected warning cases before implementation. Reverting production code against the final test file is not a reproduction of that intermediate state: the later default-logger test references the added helper and fails to compile. The fresh behavioral mutations below provide independently reproducible regression evidence without asserting an unrecorded no-op stub.
- GREEN: the same regression command passed after implementation.
- Final related suites: `go test ./cmd/cli ./internal/config/... -count=1` passed, including all 24 table cases and the default-logger test.
- `go vet ./...` passed.
- Manual mutation: temporarily attributing an explicit flag to `TS_TARGET` made `go test ./cmd/cli -run 'TestConnectProfileTargetOverrideWarning/(text|json)/flag_overrides_environment_and_YAML' -count=1` fail in both formats. The correct attribution was restored before the final passing suites.
- `git diff --check` passed.
- Git Bash: `cd scripts && bash check-doc-authkey.sh` passed (87 files).
- Local lint is unavailable: the installed v2.12.2 was built with Go 1.26 and cannot analyze this Go 1.27.1 module; isolated v2.14.0 execution exited `0xffffffff`. The owner reports Cortex detection and explicitly prohibits further local golangci-lint execution, including go-run equivalents. No endpoint settings were changed or bypassed.
- Implementation CI at `1fe82ad` completed: Linux `test` (`go test -v -race -coverprofile=coverage.out ./...`), Windows/macOS tests, lint, security, hygiene, smoke, all six build-matrix legs and GitGuardian passed. The PR-Agent `review` job was cancelled after merge; CodeRabbit's quota notice is not review coverage. These results belong to #489, not to any later follow-up commit. Native Windows race testing was not run, consistent with the existing Windows CI job.
- All command tests use temporary files, synthetic credentials and a stub runner. No live profile, credential, listener, SSH session or installed executable was modified.

## Decisions made during implementation

- Preserve the original target from the existing profile read; do not reread mutable profile storage merely to explain the override.
- Retain `TS_TARGET` attribution for both `.env` and process values. Existing tests explicitly pin `.env` overwriting process values despite a contradictory `runConnect` comment; this pre-existing discrepancy is recorded under [#399](https://github.com/mlorentedev/ts-bridge/issues/399#issuecomment-6349962198), not changed here.
- Lesson 044 is reserved by the independent host PR #485; this change uses 045. The shared lessons index is documentation bookkeeping, not a host-code change.

## Independent review and owner disposition - 2026-10-23

The original round-1 NaN report is preserved unchanged in `review-round1.md`, with
its matching `review-request-round1.json` (moved byte-identically when round 2 took
`review.md`; see below). Reviewer `nan/deepseek-v4-flash` returned **FAIL** at
`1fe82ad`, despite finding no functional defect. That verdict is not replaced by
the implementer's dispositions below. Archival remains blocked until a fresh,
passing independent verdict exists.

The owner explicitly confirmed on 2026-10-23 that they deliberately merged #489.
They authorized this separate evidence/test follow-up while preserving the
existing `master` history, the independent report and the spec contract. No
automatic merge, release, history rewrite or spec archive is authorized.

| Finding | Disposition | Evidence / boundary |
|---|---|---|
| F1: attribution trailer in merged history | Retained record; no history rewrite or attribution-suppression gate added in this follow-up. | The owner required existing history to remain intact. The original objection remains visible in the signed FAIL report; this disposition does not claim independent approval or a product defect. |
| F2: merge before review completed | Owner-confirmed deliberate merge; disclosed separately from review coverage. | #489 merged as `ca636e4`. The earlier triage was an accurate pre-merge snapshot, not a claim that review completed. PR-Agent was cancelled; CodeRabbit produced only a quota notice. No agent merge grant is invoked. |
| F3: pending-CI statement | Applied. | The test-status paragraph now distinguishes completed #489 CI from cancelled review and future follow-up checks. Contract checkboxes remain unchanged; completed historical CI is recorded here. |
| F4: real logger / quiet / JSON-file coverage | Applied in tests. | `TestConnectTargetOverrideProductionLogger` uses the real `InitLogger` in both requested formats with `--quiet`, temporary HOME/LOCALAPPDATA and synthetic profiles. It checks one JSON file record before the runner and one text console warning before a runner marker. The test closes the logger and restores globals. |
| F5: differently cased target strings | Declined normalization in this diagnostic-only change. | AC2 covers equal configured target values, not DNS resolution or endpoint equivalence. Distinct raw values are intentionally diagnosed; changing normalization/precedence is outside the proposal. Existing equal-value cases remain covered. |
| F6: future source-attribution drift | No speculative refactor. | The existing precedence table and killed competing-source mutation independently guard the labels. A future precedence change must update the resolver and these named tests together; no current mismatch was found. |
| F7: fallback ordering and absent profile target | Applied in tests. | `TestConnectTargetOverrideDefaultLogger` now exercises the full command with `LoggerInit == nil` and checks the warning inside the runner. `TestConnectProfileTargetOverrideWarning/{text,json}/profile_without_target` verifies no warning when only the overriding source supplies a target. |

The review's additional logger-lifetime question does not establish a production
leak: `run` in `cmd/cli/run.go` already defers `loggingInstance.Close()`. A stub
runner does not execute that lifecycle, so the new logger test explicitly closes
its instance. No production change or leak ticket is justified by that probe.

### Fresh follow-up regression evidence

- Added two absent-profile-target cases, strengthened the existing default-logger test without removing its warning assertions, and added two production-logger cases.
- `go test ./cmd/cli -run 'TestConnectProfileTargetOverrideWarning|TestConnectTargetOverrideDefaultLogger|TestConnectTargetOverrideProductionLogger' -count=1` passed before mutation testing.
- Moving the warning after the runner with a temporary `defer` made `go test ./cmd/cli -run '^TestConnectTargetOverrideDefaultLogger$' -count=1` fail on the missing warning inside the runner.
- Removing the empty-profile-target guard made `go test ./cmd/cli -run 'TestConnectProfileTargetOverrideWarning/(text|json)/profile_without_target' -count=1` fail in both formats with an unexpected warning.
- Disabling the file side of the combined production logger made `go test ./cmd/cli -run '^TestConnectTargetOverrideProductionLogger$' -count=1` fail in both formats on the missing JSON file record.
- All three mutations were restored. No production change is included, and `proposal.md`, `tasks.md` and `features.json` remain unchanged.
- After restoring the mutations, `go test ./cmd/cli ./internal/config/... ./internal/logging -count=1`, `go vet ./...` and `git diff --check` passed. The normalized contract digests still match the independent review request.
- No local golangci-lint, wrapper, go-run equivalent or endpoint-protection change was used.
- Follow-up CI at `d1bc868` (#496) completed: Linux `test` (`go test -v -race ./...`), `test-windows`, `test-macos`, `lint`, `security`, `hygiene`, `smoke`, all six `build-matrix` legs and GitGuardian passed. The PR-Agent `review` job was cancelled and is not review coverage. #496 merged as `fd6e4e1`.

## Independent review round 2 and owner disposition - 2026-10-23

Reviewer `nan/deepseek-v4-flash` re-reviewed `d1bc868` with unchanged contract
digests and returned **FAIL**. The signed report is `review-round2.md` with its matching
`review-request-round2.json` (moved when round 3 took `review.md`). It reproduced AC1-AC3, the related suites, `go vet` and four
killed mutation classes, and found no functional defect. Its only Major is the
commit coauthorship trailer. The round-2 verdict is not replaced by the
dispositions below, which the owner approved on #496
([triage](https://github.com/mlorentedev/ts-bridge/pull/496#issuecomment-6374809185)).

| Finding | Disposition | Evidence / boundary |
|---|---|---|
| Major: coauthorship trailer on `d1bc868` | Owner-accepted; retained. | The owner chose to keep the attribution. No amend, force-push, history rewrite or suppression gate was performed. |
| Minor: stale pending-CI statement | Applied. | The test-status section above now records the completed `d1bc868` CI snapshot. |
| Minor: differently cased target values | Declined, as in round 1. | AC2 specifies configured-value equality; a distinct raw value is intentionally diagnosed. No normalization or contract change. |
| Minor: future source-attribution drift | Declined, as in round 1. | No current mismatch; the precedence table and killed mutations pin today's labels. |

Archival remains blocked: both signed verdicts are FAIL, and owner acceptance is
not a passing independent verdict or an archive waiver.

## Owner disposition update - 2026-10-28

The owner revisited the round-2 Major and chose prevention over acceptance. The remedy the
review proposed (amend `d1bc868` before it reached `master`) is no longer available: #496 merged
as `fd6e4e1` and #497 as `aa3e8c6`, both carrying the trailer, and the owner's no-rewrite rule
keeps `master` history as it is. What the reviews asked for that *is* still possible is now done:

| Finding | Updated disposition | Evidence / boundary |
|---|---|---|
| Major (rounds 1-2): AI coauthorship trailer, no gate catches it | Prevention gate shipped; merged history retained. | Ticket #506 (`CI-358-ai-trailer-gate`), merged in #510 as `1553b0d`: `Repo hygiene` runs `scripts/check-ai-trailers.sh` over every pull request's `base..head` and fails on a `Co-authored-by` trailer naming an AI agent. Its own run on #510 reported `check-ai-trailers: OK (2 commits)` and `test-ai-trailers: OK (bash zsh)`. Against history, `bash scripts/check-ai-trailers.sh 813d898~5 813d898` lists the trailered commits including `fd6e4e1`, so the gate would have refused #496. The owner has instructed the agent to stop appending the trailer. `fd6e4e1`, `aa3e8c6` and `ca636e4` stay as they are. |
| Minor: differently cased target values (round-2 F3) | Ticketed: #508. | Round 2 graded it REAL; it is now tracked instead of declined. Still a diagnostic false positive with no behaviour change, outside AC2's raw-value contract. |
| Minor: future source-attribution drift (round-2 F4) | Declined, as in rounds 1-2. | No current mismatch; the precedence table and killed mutations pin today's labels. A ticket would track a hypothetical. |

The contract set (`proposal.md`, `tasks.md`, `features.json`) is unchanged, so a round-3 review
reads the same digests. Archival waits for that verdict.

## Independent review round 3 - 2026-10-28

Reviewer `nan/deepseek-v4-flash` returned **PASS** at `ff6f676` against unchanged contract
digests (`review.md` with `review-request.json`). No Blocker or REAL Major; every open item is a
minor with a disposition below.

| Finding | Disposition | Evidence / boundary |
|---|---|---|
| F1 Minor: the `invalid target` row is vacuous (the error path never calls `LoggerInit`) | Applied (tests). | The table test now routes `logger` and `slog.Default()` into the captured buffer before `Execute`, so a warning emitted before validation fails the silence check. Mutant M5 (an extra `warnConnectTargetOverride` before `config.Merge`) passed `.../(text\|json)/invalid_target` before the change and fails both after, with `unexpected override warning`. `go test ./cmd/cli ./internal/config/... ./internal/logging -count=1` and `go vet ./cmd/cli` pass. Test-only; no production change. |
| F2 Minor: two `tasks.md` boxes and `features.json` states understate completion | Recorded, not edited. | Both tasks are done: CI lint and Linux race passed on #489 and #496, and #489 carried `Closes #417`. The boxes and `pending` states stay as they are because editing the contract set would invalidate this verdict's digests; the harness, not the implementer, sets `passing`. |
| F3 Minor: round 2's signed report would leave the repo | Applied. | Committed as `review-round2.md` / `review-request-round2.json`; the round-2 section above now points at them. |
| F4 Minor: case-differing target values warn | Ticketed: #508. | Outside AC2's raw-value contract. |
| F5 Minor (SPECULATIVE): source label re-derived at the call site | Declined, as in rounds 1-2. | No current mismatch. |
| F6 Minor: governance residual | Recorded. | `master` keeps `ca636e4`, `fd6e4e1` and `aa3e8c6` with the trailer by owner decision. The #510 gate covers pull-request commits only: it skips push runs and does not read PR or issue bodies. |

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-045-profile-target-source.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: existing precedence and profile model are unchanged.
- [x] New pattern candidate for `00_meta/patterns/`? no: no cross-project recurrence established.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-030-profile-target-source/` -> `specs/archive/BUG-030-profile-target-source/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
