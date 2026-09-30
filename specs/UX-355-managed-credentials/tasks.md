---
tags: [spec, tasks, templates]
created: "2026-09-29"
---

# Tasks - UX-355-managed-credentials

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created from ADR-015 branch: `feat/managed-credentials`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

> Replace these with the actual steps for this feature. Keep them small (one commit each) and in TDD order.
> The `[P]` / `[AC<n>]` markers are optional — see the legend above. Behaviors 1 and 2 below are independent, so their *first* test task carries `[P]`.

- [x] [P] [AC5] Add failing cross-platform tests for `config.CredentialStoreDir()` beside `ProfileStorePath()`.
- [x] [AC1] [AC5] Add failing `internal/credential` store tests: valid names/keys, atomic set, overwrite refusal/force, list/status/remove, missing key, and no secret in errors.
- [x] [AC1] [AC5] Implement the file-backed credential store with Unix owner-only modes and Windows `icacls` hardening.
- [x] [AC2] Add failing profile-store tests for `credential`, backward-compatible YAML, binding/clearing, preservation on import/set, and secret-free descriptors.
- [x] [AC2] Implement the additive profile credential reference and safe update methods. Open foundation PR referencing #368.
- [x] [AC1] [AC4] Add failing production command-tree tests for `auth set/status/list/remove`, masked/stdin input, overwrite behavior, shared-reference refusal, and output redaction.
- [x] [AC1] [AC4] Implement the Cobra `auth` command tree and register it in `NewRootCmd`.
- [x] [AC3] Add failing production command tests for connect/browser profile credential resolution and explicit-source precedence.
- [x] [AC3] Implement one shared profile credential resolver used by connect and browser.
- [ ] [AC1] [AC2] [AC3] Update README, `.env.example`, CLI reference, configuration docs, ADR-012/014 amendments, and security audit. Open integration PR closing #355.
- [x] [AC1] [AC2] [AC3] [AC4] [AC5] Run build, vet, targeted Go tests, lint, gosec, and site build.

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` with a non-vacuous verification command
- [x] Type checks pass
- [x] Lint passes
- [x] No unrelated changes in the diff (no scope creep)
- [x] `verification.md` filled in
- [ ] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/UX-355-managed-credentials/features.json`):

```json
[
  {
    "id": "UX-355-managed-credentials-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
