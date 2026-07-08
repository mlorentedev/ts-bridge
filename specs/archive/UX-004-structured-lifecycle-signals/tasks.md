---
tags: [spec, tasks]
created: "2026-07-08"
---

# Tasks - UX-004-structured-lifecycle-signals

> TDD order. One combined PR (decision: #203 + #204 ship together as one "make the bridge
> lifecycle observable" change — tightly coupled, shared line protocol, small production diff).
> Closes #203 and #204.

## Setup

- [x] Branch created from master: `feat/structured-lifecycle-signals`
- [x] `proposal.md` complete, acceptance criteria testable
- [x] Open question resolved: `escapeSignalDetail` defined (escape `"`, collapse newlines)

## Implementation

### Signal helpers (shared grammar) — `cmd/cli/signal.go`

- [x] [P] [AC5] Failing test for the escaping helper: `detail` with embedded `"` and `\n` renders as a single parseable line (`TestEscapeSignalDetail`)
- [x] [AC5] Implement `escapeSignalDetail` (escape `"`, collapse newlines) + `formatErrorLine`/`formatReadyLine`
- [x] Extract `emitReady`/`emitError` + `reason*` consts so READY and ERROR share the grammar

### READY signal (#203)

- [x] [P] [AC1] Failing test: READY line uses the **actual** `listener.Addr()` (real `127.0.0.1:0` listener, port ≠ 0) — `TestEmitReady_UsesBoundAddr`
- [x] [AC1] Emit `READY` in `Run` after the banner, before the health server (BUG-010 stdout-race grouping)
- [x] [AC2] Add `--quiet` flag to `connectCmd`; resolve onto `cfg.Quiet`; gate `writeStartupBanner` on it (READY still emits)
- [x] [AC2] Failing test: `--quiet` suppresses banner but the `READY` line remains — `TestWriteStartupBanner_QuietSuppresses`

### ERROR signal (#204)

- [x] [AC3] Failing table-driven test: reason token per category — `TestDiagnoseTailscaleInitError_Reason`
- [x] [AC3][AC4] Extend `diagnoseTailscaleInitError` to return a stable `reason` token; emit `ERROR` to stderr in `initTailscale`
- [x] [AC4] `unknown` fallback never silent (tested via `unrecognized` case + always-emit in `initTailscale`)

## Closing

- [x] Every acceptance criterion covered by ≥1 test
- [x] Every acceptance criterion has a `features.json` entry with a non-vacuous verification command
- [x] `go test ./... -count=1` green (CI runs `-race` on Linux)
- [x] `go vet ./...` clean (goconst: `reason` tokens are consts)
- [x] README + flag help document the `READY`/`ERROR` grammar and `--quiet` (`.env.example` n/a — no env var)
- [x] `verification.md` filled in
- [ ] PR opened referencing this spec folder, `Closes #203, Closes #204`

## Machine-readable features

See sibling `features.json`. Pass-state is harness-gated — the agent never writes `"state": "passing"`.
