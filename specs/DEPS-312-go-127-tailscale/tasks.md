---
tags: [spec, tasks, templates]
created: "2026-10-09"
---

# Tasks - DEPS-312-go-127-tailscale

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch `chore/go-127-tailscale` created from `origin/master` in an external worktree.
- [x] `proposal.md` defines observable acceptance criteria.
- [x] Upgrade scope and compatibility risks are resolved.

## Implementation

- [x] [AC2] Write `scripts/tests/test-go-toolchain.sh` to require every `actions/setup-go` step to use `go-version-file: go.mod`; observe red on the 1.26 pins and on a conflicting pin fixture.
- [x] [AC2] Switch `.github/workflows/{ci,release,mutation,repo-hygiene}.yml` to module-derived toolchains; observe green on the real workflows and rejection of stale fixtures.
- [x] [AC1] Incorporate the dependency upgrade from PR #431 (`go.mod` and `go.sum`), using Go 1.27.1; run `go mod tidy` and compare the module files with PR #431.
- [x] [AC3] Pin a Go-1.27-compatible linter in `.github/workflows/ci.yml`; run `go build ./...`, `go vet ./...`, `go test ./...`, and the pinned linter.
- [x] [AC4] Update `AGENTS.md` toolchain/linter instructions and verify the declared versions match.
- [x] [AC1] [AC2] [AC3] [AC4] Run CI guards and `git diff --check`; record results in `verification.md`.

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by evidence or awaiting CI.
- [x] Every criterion has a non-vacuous `features.json` verification.
- [x] Type checks pass.
- [x] Lint passes.
- [x] No unrelated changes in the diff.
- [x] `verification.md` filled in.
- [ ] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/DEPS-312-go-127-tailscale/features.json`):

```json
[
  {
    "id": "DEPS-312-go-127-tailscale-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
