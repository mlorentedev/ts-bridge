---
tags: [spec, tasks]
created: "2026-07-08"
---

# Tasks - UX-004-structured-lifecycle-signals

> TDD order. One combined PR (decision: #203 + #204 ship together as one "make the bridge
> lifecycle observable" change — tightly coupled, shared line protocol, small production diff).
> Closes #203 and #204.

## Setup

- [ ] Branch created from master: `feat/structured-lifecycle-signals`
- [ ] `proposal.md` complete, acceptance criteria testable
- [ ] Open question resolved: `detail=` escaping helper defined before implementation

## Implementation

### Signal helpers (shared grammar)

- [ ] [P] [AC5] Write failing test for a `formatSignal`/escaping helper: `detail` with embedded `"` and `\n` renders as a single parseable line
- [ ] [AC5] Implement the escaping helper (quote + collapse newlines) in `cmd/cli/run.go`
- [ ] Refactor: extract the `KEY=value` line builder so READY and ERROR share it

### READY signal (#203)

- [ ] [P] [AC1] Write failing test: on successful `Run`, a `READY local=<bound> target=<target>` line is written to stdout, using the **actual** `listener.Addr()` (test with an auto-assigned port so requested ≠ bound)
- [ ] [AC1] Emit the `READY` line in `Run` after `net.Listen` + `MarkReady`, before `AcceptLoop`
- [ ] [AC2] Add `--quiet` flag to `connectCmd`; thread through config; gate `printBanner` + "Waiting…" on it (READY still emits)
- [ ] [AC2] Write failing test: `--quiet` suppresses banner but the `READY` line remains

### ERROR signal (#204)

- [ ] [AC3] Write failing table-driven test: `initTailscale` failure emits `ERROR reason=<token> detail="…"` to stderr for `bad_authkey`, `control_plane_unreachable`, `unknown`
- [ ] [AC3][AC4] Extend `diagnoseTailscaleInitError` to also return a stable `reason` token (closed set); emit the `ERROR` line to stderr on the init-failure path in `initTailscale`
- [ ] [AC4] Verify the `unknown` fallback path is never silent (test an unclassified error string)

## Closing

- [ ] Every acceptance criterion covered by ≥1 test
- [ ] Every acceptance criterion has a `features.json` entry with a non-vacuous verification command
- [ ] `go test ./... -count=1` green (CI runs `-race` on Linux)
- [ ] Lint passes (goconst: reuse consts for repeated `reason` tokens / literals)
- [ ] `.env.example` / README / flag help document the `READY`/`ERROR` grammar and `--quiet`
- [ ] `verification.md` filled in
- [ ] PR opened referencing this spec folder, `Closes #203, Closes #204`

## Machine-readable features

See sibling `features.json`. Pass-state is harness-gated — the agent never writes `"state": "passing"`.
