---
tags: [spec, tasks, templates]
created: "2026-06-24"
---

# Tasks - STATE-001

> TDD order. One task = one focused commit. Tick as you go.

## Setup

- [x] Branch created from master: `fix/state-dir-per-user-default`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

> TDD order, one commit each.

- [x] Test: `StateDirForPlatform()` is absolute on every OS; honors `$XDG_STATE_HOME`; falls back to
      `~/.local/state` / temp; never CWD-relative even with all env unset
- [x] Implement `internal/config/statedir.go` `StateDirForPlatform()`
- [x] Test: `Merge` default (no state-dir input) → `StateDir == StateDirForPlatform()`, absolute,
      not `./ts-state`
- [x] Wire `StateDirForPlatform()` as the manual-mode default in `merge.go` (and legacy `config.go`)
- [x] Test: auto-instance ephemeral branch resolves under temp (not CWD), `EphemeralState == true`
- [x] Fix `merge.go` auto-ephemeral branch to use a temp dir instead of `./ts-state`
- [x] Test: explicit `--state-dir` / `TS_STATE_DIR` / yaml override is preserved verbatim
- [x] Test: resolved relative state dir emits a `Warn`; absolute does not
- [x] Implement warn-if-relative guard (in `config` `Merge`/`LoadConfig`, covering both paths)
- [x] Update `config_test.go` legacy assertion (`./ts-state` → `StateDirForPlatform()`)
- [x] Docs: getting-started, configuration, cli-reference, troubleshooting reflect new default

## Closing

- [ ] Every acceptance criterion is covered by ≥1 test
- [ ] Every acceptance criterion has a matching `features.json` entry with a real verification command
- [ ] `go build ./... && go vet ./... && go test ./...` green (run sequentially — never two `go` at once)
- [ ] No unrelated changes in the diff (no scope creep)
- [ ] `verification.md` filled in
- [ ] PR opened referencing this spec folder

## Machine-readable features

See sibling `features.json`. Pass-state gating: the agent CANNOT write `"state": "passing"` — only
the harness, after running `verification` and capturing exit 0, may set that terminal state.
