---
tags: [spec, tasks, templates]
created: "2026-11-14"
id: "LOG-788-rotator-shared-file"
type: spec-tasks
status: active
owner: manu
---

# Tasks - LOG-788-rotator-shared-file

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created from master: `fix/issue-788-log-rotator-shared-file`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions" (all accepted with reasons)

## Implementation

- [x] [AC1] Failing test: two rotators on one dir, close one, the other keeps writing to an existing file
- [x] [AC1] `Close` stops compressing
- [x] [P] [AC2] Failing test: compressing a day whose `.gz` exists appends a member
- [x] [AC2] `compressDay`: claim by rename, append a gzip member, remove the claim
- [x] [P] [AC3] [AC4] Failing test: a rollover compresses past days, never today
- [x] [AC3] [AC4] One `rotateLocked(today)` path for `Open` and `Write`: open today, compress every past `.log`, clean up

## Closing

- [x] No ADR needed: ADR-018 already decided the rule
- [x] Every acceptance criterion is covered by a test and a `features.json` entry
- [x] `go test -race ./...`, `golangci-lint run ./...` green
