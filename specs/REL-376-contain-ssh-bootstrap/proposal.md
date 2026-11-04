---
id: "REL-376-contain-ssh-bootstrap"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-11-04"
issue: "mlorentedev/ts-bridge#376"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# REL-376-contain-ssh-bootstrap

## Why

The SSH control bootstrap (CLI-363) starts `ssh -N -D`. On a graceful shutdown that child is
stopped. Not every exit is graceful, though: after a panic, a `kill -9` or Task Manager's End task,
`ssh` keeps running and holds the bootstrap SOCKS port. The next `connect --bootstrap-ssh` then fails
to bind. Go attaches no child to the parent's lifetime by default. Found by the CLI-363 adversarial
review (#376).

## What

The bootstrap ties `ssh` to ts-bridge's lifetime where the OS allows it:

- **Linux**: `SysProcAttr.Pdeathsig = SIGKILL` is set before the start.
- **Windows**: after the start, `ssh` is assigned to a Job Object with
  `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`. The job handle is released only after `Wait` returns, so the
  graceful path keeps today's order (cancel, kill, wait). On an abrupt exit, the kernel closes the
  handle and the job's members die.
- **Other systems (macOS)**: no change. There is no equivalent, and the runbook documents the boundary.

Containment is best effort. A refusal (access denied, or a parent job that forbids nesting) never
stops the bootstrap, which then runs exactly as before.

## Out of scope

- A watcher process for macOS.
- Covering a process `ssh` spawns before the Windows assignment (`ProxyCommand`). Closing that
  window needs a suspended start, which `os/exec` does not offer.

## Risks / open questions

- Pdeathsig follows the forking thread, not the process. Go retires a thread only when a
  `LockOSThread` goroutine exits, and the bootstrap never does that. Documented in code.
- Nothing local exercises the Windows path. Its real-kill test runs only in `test-windows`, so that
  CI job is the evidence.

## Acceptance criteria

- [ ] AC1: On Linux and Windows, killing the parent outright also ends a contained child within 5s.
- [ ] AC2: A failing containment does not fail `Start`, and the tunnel closes normally.
- [ ] AC3: The containment is applied to the started process and released only after it is waited
  for (graceful order unchanged).
- [ ] AC4: The runbook documents what each OS covers and the forced-termination boundary.

## References

- Issue #376; CLI-363 (`internal/bootstrapssh`); `docs/runbooks/guide-control-bootstrap.md`
