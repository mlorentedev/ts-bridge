---
tags: [spec, tasks]
created: "2026-11-14"
id: "AGENT-001-local-control-plane"
type: spec-tasks
status: active
owner: manu
---

# Tasks - AGENT-001-local-control-plane

The issue is split into five PRs of at most ~300 executable lines each (tests, lockfiles and generated
files excluded), every one test-first. The split follows the seams of ADR-018: the agent and its Unix
transport first, because every later PR rests on them; then the Windows transport; then what runs
behind the socket; then what is stored; then the first client. PRs 1 to 4 say `Refs #782`; PR 5 says
`Closes #782`.

## PR 1 - single-instance agent with an identity-checked Unix API (`Refs #782`)

### Setup

- [x] Branch from master: `feat/issue-782-agent-local-api`
- [x] `proposal.md` reconciled with the merged ADR-018 (base from the user database, uid-only Linux
  socket path, `/v1/meta`, error envelope, client-side server check, Windows split out)

### Implementation

- [ ] Move the advisory file lock to `internal/oslock`; `internal/profile` uses it (existing lock tests stay green)
- [ ] [AC2] Failing test, then the paths: base from the user database per OS, Linux socket directory
  from the uid, macOS `<base>/run`; directory rules (0700, owner, symlink) and the `sun_path` limit
- [ ] [AC1] Failing test, then `Start`: lock, second starter refused, stale socket removed under the
  lock, non-socket never removed
- [ ] [AC3] Failing test, then `Authorize` and the identifying listener (fake peers, accept loop
  survives a refusal); Unix peer credentials (Linux, macOS)
- [ ] [AC4] Failing test, then the client's directory and server-identity checks
- [ ] [AC5] `GET /v1/meta`, `GET /v1/status`, the error envelope, `Serve`/`Close`, `HTTPClient`, and
  `ts-bridge agent` (foreground); Windows refuses to start with a message naming the follow-up
- [ ] ADR-007 package table: `oslock`, `agent`, and the `profile` row

### Closing

- [ ] `go test -race ./...`, `golangci-lint run`, `gosec ./...`, `GOOS=windows go vet ./...`, `GOOS=darwin go vet ./...`
- [ ] Mutation: `Authorize` always true, stale socket removed before the lock, socket mode left at the
  umask, client server check skipped - each fails a test
- [ ] `verification.md` evidence filled once CI is green

## PR 2 - Windows transport (`Refs #782`, in review)

- [x] [AC6] Named pipe `\\.\pipe\ts-bridge-agent-<SID>`, SDDL `D:P(A;;GA;;;<SID>)`,
  `FILE_FLAG_FIRST_PIPE_INSTANCE`, `PIPE_REJECT_REMOTE_CLIENTS`; a name already owned fails and is
  never retried under another name
- [x] [AC6] Per-connection client token SID check; client opens with
  `SECURITY_SQOS_PRESENT | SECURITY_IDENTIFICATION` and checks `GetNamedPipeServerProcessId`'s token SID
- [x] `go.mod`: `tailscale/go-winio` loses its `// indirect` marker (operator-approved); `go.sum` unchanged

## PR 3 - session supervisor (`Refs #782`, not started)

- [ ] [AC7] `connect --lifeline <fd|handle>` (hidden): `stop` line drains, EOF drains on Unix; stdin is
  the null device
- [ ] [AC7] Spawn: working dir, allow-listed environment, `.env` off, `--quiet`, `READY`/`ERROR` parsing;
  Windows suspended + Job Object + resume; duplicate local address refused; stable hostname
- [ ] Per-profile lock holder metadata (kind, pid, start) and the bounded wait for a previous
  generation's lock
- [ ] `/v1/sessions` list, start, stop, status; the skew check on mutations

## PR 4 - recovery and profiles (`Refs #782`, not started)

- [ ] [AC8] State machine, budget of three, persisted desired state, a minute of `ready` resets the count
- [ ] [AC8] `/v1/profiles` list and import through the ADR-015 transition with an explicit `force`;
  audit line per mutation

## PR 5 - session CLI (`Closes #782`, not started)

- [ ] [AC9] `ts-bridge session list|start|stop|status` over `agent.HTTPClient`
- [ ] [AC9] `connect --profile X` and an agent session of X refuse each other, each naming the other
- [ ] Tick every AC in `verification.md`; archive the spec after review

## Notes

- `features.json` lists the features whose tests exist at each PR: `check-features-selectors.sh`
  fails a selector that matches no test, so each PR appends its own.
