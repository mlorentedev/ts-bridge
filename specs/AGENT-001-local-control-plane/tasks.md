---
tags: [spec, tasks]
created: "2026-11-14"
id: "AGENT-001-local-control-plane"
type: spec-tasks
status: active
owner: manu
---

# Tasks - AGENT-001-local-control-plane

The issue is split into four PRs of at most ~300 executable lines each (tests, lockfiles and generated
files excluded), every one test-first. The split follows the seams of ADR-018: identity and transport
first, because every later PR rests on them; then what runs behind the socket; then what is stored;
then the first client. PRs 1 to 3 say `Refs #782`; PR 4 says `Closes #782`.

## PR 1 - single-instance agent with an identity-checked local API (`Refs #782`)

### Setup

- [x] Branch from master: `feat/issue-782-agent-local-api`
- [x] `proposal.md` complete; Windows pipe dependency decided (`tailscale/go-winio`, already in `go.mod`)

### Implementation

- [ ] Move the advisory file lock to `internal/oslock`; `internal/profile` uses it (existing lock tests stay green)
- [ ] [AC1] Failing test, then `Start`: lock, second starter refused, stale socket removed under the lock, non-socket never removed
- [ ] [AC2] Failing test, then the state-directory rules (default per OS, 0700, owner, symlink, socket mode, path length)
- [ ] [AC3] Failing test, then `Authorize` and the identifying listener (fake peers, accept loop survives a refusal)
- [ ] [AC3] [AC4] Unix peer credentials (Linux, macOS), Windows pipe, SDDL and token SID check
- [ ] [AC4] `GET /v1/status`, `Serve`/`Close`, `HTTPClient`, and `ts-bridge agent` (foreground)
- [ ] ADR-007 package table: `oslock`, `agent`, and the `profile` row

### Closing

- [ ] `go test -race ./...`, `golangci-lint run`, `gosec ./...`, `GOOS=windows go vet ./...`, `GOOS=darwin go vet ./...`
- [ ] Mutation: `Authorize` always true, stale socket removed before the lock, socket mode left at the umask - each fails a test
- [ ] `verification.md` evidence filled once CI is green

## PR 2 - session supervisor (`Refs #782`, not started)

- [ ] [AC5] Child process re-exec of `connect --profile <name>`, with a lifetime tie per OS (`Pdeathsig` on
  Linux, a Job Object with `KILL_ON_JOB_CLOSE` on Windows, a lifeline pipe closed on EOF on macOS)
- [ ] [AC5] Desired-running set in the state directory: restore after an unexpected stop, keep a user stop
- [ ] [AC5] `/v1/sessions` list, start, stop, status

Reason for the cut: the lifetime tie is three OS-specific mechanisms and the recovery set is its own
state machine; together they fill a PR without touching the credential code.

## PR 3 - profiles and discovery (`Refs #782`, not started)

- [ ] [AC6] `/v1/profiles` list; import of a `tsb://` descriptor with its credential through `BindCredential`
- [ ] [AC6] Discovery endpoint over `internal/discover`; a test that no secret reaches any file but the store

## PR 4 - session CLI (`Closes #782`, not started)

- [ ] [AC7] `ts-bridge session list|start|stop|status` over `agent.HTTPClient`
- [ ] [AC7] `connect --profile X` and an agent session of X refuse each other, each naming the other
- [ ] Tick every AC in `verification.md`; archive the spec after review

## Notes

- `features.json` lists the features whose tests exist at each PR: `check-features-selectors.sh`
  fails a selector that matches no test, so each PR appends its own.
