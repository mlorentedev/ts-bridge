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

The original NaN report is preserved unchanged in `review.md`, with its matching
`review-request.json`. Reviewer `nan/deepseek-v4-flash` returned **FAIL** at
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
- No local golangci-lint, wrapper, go-run equivalent or endpoint-protection change was used. Cross-platform/race/lint results for this follow-up must come from its own CI, not the earlier implementation head.

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
