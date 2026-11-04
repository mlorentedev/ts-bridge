---
tags: [spec, verification]
created: "2026-11-04"
id: "REL-376-contain-ssh-bootstrap"
type: spec-verification
status: active
owner: manu
---

# Verification - REL-376-contain-ssh-bootstrap

## Evidence

- [ ] AC1 → `TestContainment_KillsChildWhenParentDies`. A helper parent contains a sleeping child
  and prints the child's pid. The test then kills the parent (SIGKILL / TerminateProcess) and polls
  until the child is gone or a zombie.
  - Linux (Docker `golang:1.27`): PASS.
  - Mutant: without Pdeathsig, the test fails with `child 4610 outlived its killed parent`.
  - Windows: `test-windows` CI only; the run is not recorded here yet.
- [ ] AC2 → `TestStartSurvivesContainmentFailure`: with containment returning `access denied`,
  `start` still returns a tunnel, and `Close` works.
- [ ] AC3 → `TestStartContainsTheStartedProcessAndReleasesAfterWait`: `cmd.Process` is set when
  containment runs, and `cmd.ProcessState` is set when it is released.
- [ ] AC4 → `docs/runbooks/guide-control-bootstrap.md`, Routing and ownership.

## Test status

- macOS: `go test -race ./...` exit 0. This proves only that the no-op path compiles and that the
  seam tests pass; it is not containment evidence.
- Linux (Docker): `go test -race ./internal/bootstrapssh/` ok. The `Contain|Pdeathsig` tests pass
  with `-v`.
- `golangci-lint run` reports 0 issues for darwin, linux and windows. `gosec ./...` exit 0.

## Decisions made during implementation

- Containment is best effort. A failure is dropped, not surfaced. The alternative, refusing to
  start, would turn a theoretical leak into a certain outage on hosts where Job Objects nest badly.
- The Windows job handle is closed in the `Wait` goroutine, not in `Close`. Every exit path
  (graceful, early exit, startup failure) then releases it after the process has ended.
- macOS stays uncontained: there is no kernel facility for this, and a watcher process is a second
  thing to leak.

## Promotion candidates

- [ ] Lesson for the repo's `docs/lessons/`? no: the per-OS facts are in the code comments and the runbook, nothing surprising was hit
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: an implementation detail of ADR-005's CLI-363 amendment
- [ ] New pattern candidate for `00_meta/patterns/`? no: one occurrence

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/REL-376-contain-ssh-bootstrap/` -> `specs/archive/REL-376-contain-ssh-bootstrap/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
