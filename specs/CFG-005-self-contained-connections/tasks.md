---
tags: [spec, tasks, templates]
created: "2026-10-15"
---

# Tasks - CFG-005-self-contained-connections

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created from current `origin/master`: `feat/self-contained-profile-connections`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No unresolved design questions block implementation; live SSH unlock remains an operator interaction

## Implementation

- [x] [P] [AC1] Add runtime round-trip/preservation tests to `internal/profile/runtime_test.go`; run `go test ./internal/profile -run Runtime -count=1` and observe the missing runtime behavior.
- [x] [AC1] Extend `internal/profile/store.go` with non-secret runtime configuration and preserve it during Set/Import/credential changes; rerun the runtime/store tests to green.
- [x] [AC2] Add production-tree tests in `cmd/cli/profile_runtime_test.go` for one-time YAML import, existing target/credential preservation, profile-only connect, precedence, and browser target separation; observe RED, then wire `init`, `connect`, shared profile resolution, and YAML local bind support; run `go test ./cmd/cli ./internal/config ./internal/profile -count=1`.
- [x] [P] [AC3] Add same-profile, cross-profile, process-exit, and production runner-order tests in `internal/profile/lock_test.go` and `cmd/cli/profile_runtime_test.go`; observe RED.
- [x] [AC3] Implement OS-backed profile ownership in `internal/profile/lock*.go` and shared command lifecycle; run `go test ./internal/profile ./cmd/cli -count=1` to green.
- [x] [P] [AC4] Add missing-client and early-exit diagnostic tests in `internal/bootstrapssh/bootstrap_test.go`; observe RED, then enhance errors without capturing terminal output; run `go test ./internal/bootstrapssh ./cmd/cli -count=1`.
- [x] [AC5] Update `docs/runbooks/guide-profile-connections.md` with one-time setup, daily foreground invocation, matching SSH-client diagnostics, duplicate/port collision behavior, and a TLS-valid service probe.
- [ ] [AC1] [AC2] [AC3] [AC4] Run the Windows package suite, build and vet plus `golangci-lint run`; cross-compile platform-specific lock implementations. Record any environment limitations, including unavailable race-detector tooling.
- [ ] [AC5] With the operator, migrate only the requested local profile using the built CLI, prove repeated configuration is unchanged, run the bridge with interactive SSH unlock, probe the real service, attempt the duplicate, and confirm unrelated profiles/processes are untouched.

## Closing

- [ ] Every acceptance criterion from `proposal.md` is covered by at least one test
- [ ] Every acceptance criterion has a matching entry in `features.json` (see below) with a non-vacuous verification command
- [ ] Type checks pass
- [ ] Lint passes
- [ ] No unrelated changes in the diff (no scope creep)
- [ ] `verification.md` filled in
- [ ] Independent review requested before archive; no archive or completion claim without live AC5 evidence
- [ ] PR opened referencing this spec folder when authorized

## Pause - 2026-10-15

The operator requested handoff and continuation on 2026-10-16. Full Windows
tests, vet, build, and six profile-ownership cross-builds passed. Lint is
uncertified: the installed tool has an older Go build version, and execution
of the session-local replacement was prevented by workstation endpoint
protection. Do not restore, rerun, rename, or relocate the flagged executable
to bypass that decision. Resume validation only through an approved channel.

No live profile migration or connection was started. AC5, independent review,
commit/push/PR authorization and archive remain open.

## Resume - 2026-10-16

The operator authorized committing and publishing the feature as a draft PR
and using the existing GitHub Actions Linux/Windows runners for automated
validation. This replaces the local-lint prerequisite with an authorized CI
validation path; it does not resolve or bypass workstation endpoint protection.
Do not execute or transfer the prevented utility, provision mesh credentials in
CI, migrate a real profile, or claim live AC5 completion.

Keep the PR draft until the remaining delivery gates are satisfied. Record CI
results against the published revision; the earlier Windows results describe
the pre-publication checkout, not a future dependency integration.

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/CFG-005-self-contained-connections/features.json`):

```json
[
  {
    "id": "CFG-005-self-contained-connections-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
