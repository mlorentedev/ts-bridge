---
tags: [spec, tasks, templates]
created: "2026-09-30"
---

# Tasks - CI-348-doc-auth-key-guard

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch supplied by the user from `master@120a6dd`: `fix/ci-348`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

- [x] [AC1] [AC2] Add committed Markdown fixtures below
      `scripts/tests/fixtures/doc-authkey/` for a clean tree, prose-only mention, inline connect,
      warned inline init, and unwarned inline init.
- [x] [AC2] Add `scripts/tests/test-doc-authkey.sh`, pointed at the wished-for
      `scripts/check-doc-authkey.sh`, and run
      `TDA_SHELLS=bash bash scripts/tests/test-doc-authkey.sh`.
      Expected RED: guard not found, exit 2.
- [x] [AC1] [AC2] Implement the minimal `scripts/check-doc-authkey.sh` that scans an optional root,
      reports file/line findings, distinguishes command examples from prose, and enforces the
      three-line init warning window.
- [x] [AC1] [AC2] Re-run the targeted suite. Expected GREEN: every fixture reports the expected
      exit code and message under bash; CI later certifies zsh.
- [x] [AC3] Add the real-tree guard and fixture suite steps to
      `.github/workflows/repo-hygiene.yml`.
- [x] [AC3] Run the real-tree guard, all repository hygiene scripts, workflow lint, and the Go
      build/test/vet/lint gates; record exact results in `verification.md`.

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` (see below) with a non-vacuous verification command
- [x] Type checks pass
- [x] Lint passes
- [x] No unrelated changes in the diff (no scope creep)
- [x] `verification.md` filled in
- [ ] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/CI-348-doc-auth-key-guard/features.json`):

```json
[
  {
    "id": "CI-348-doc-auth-key-guard-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
