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
- RED: `go test ./cmd/cli -run TestConnectProfileTargetOverrideWarning -count=1` failed on the ten expected warning cases: no warning before the runner. Existing no-warning cases passed.
- GREEN: the same regression command passed after implementation.
- Final related suites: `go test ./cmd/cli ./internal/config/... -count=1` passed, including all 24 table cases and the default-logger test.
- `go vet ./...` passed.
- Manual mutation: temporarily attributing an explicit flag to `TS_TARGET` made `go test ./cmd/cli -run 'TestConnectProfileTargetOverrideWarning/(text|json)/flag_overrides_environment_and_YAML' -count=1` fail in both formats. The correct attribution was restored before the final passing suites.
- `git diff --check` passed.
- Git Bash: `cd scripts && bash check-doc-authkey.sh` passed (87 files).
- Local lint is unavailable: the installed v2.12.2 was built with Go 1.26 and cannot analyze this Go 1.27.1 module; isolated v2.14.0 execution exited `0xffffffff`. The owner reports Cortex detection and explicitly prohibits further local golangci-lint execution, including go-run equivalents. No endpoint settings were changed or bypassed.
- Linux race tests and all-platform CI remain pending PR checks; native Windows race testing was not run, consistent with the existing Windows CI job.
- All command tests use temporary files, synthetic credentials and a stub runner. No live profile, credential, listener, SSH session or installed executable was modified.

## Decisions made during implementation

- Preserve the original target from the existing profile read; do not reread mutable profile storage merely to explain the override.
- Retain `TS_TARGET` attribution for both `.env` and process values. Existing tests explicitly pin `.env` overwriting process values despite a contradictory `runConnect` comment; this pre-existing discrepancy is recorded under [#399](https://github.com/mlorentedev/ts-bridge/issues/399#issuecomment-6349962198), not changed here.
- Lesson 044 is reserved by the independent host PR #485; this change uses 045. The shared lessons index is documentation bookkeeping, not a host-code change.

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
