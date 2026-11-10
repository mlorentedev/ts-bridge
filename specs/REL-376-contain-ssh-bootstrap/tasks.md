---
tags: [spec, tasks]
created: "2026-11-04"
id: "REL-376-contain-ssh-bootstrap"
type: spec-tasks
status: active
owner: manu
---

# Tasks - REL-376-contain-ssh-bootstrap

## Setup

- [x] Branch from master: `fix/issue-376-contain-ssh-bootstrap`
- [x] `proposal.md` complete; best-effort contract decided

## Implementation

- [x] [AC2] [AC3] Failing seam tests: `TestStartSurvivesContainmentFailure`,
  `TestStartContainsTheStartedProcessAndReleasesAfterWait`
- [x] [AC1] Failing real-kill test `TestContainment_KillsChildWhenParentDies` (linux, windows) and
  `TestPrepareContainment_SetsPdeathsig` (linux)
- [x] Implement `contain_{linux,windows,other}.go` and wire `prepare`/`contain` into `start`
- [x] [AC4] Runbook boundary per OS

## Closing

- [x] macOS `go test -race ./...`; Linux in Docker (`golang:1.27`) for the package with `-race`
- [x] `golangci-lint` for darwin, linux and windows; `gosec ./...`
- [x] Mutation (Linux): dropping Pdeathsig makes the child outlive its parent
- [x] `test-windows` CI log shows `TestContainment_KillsChildWhenParentDies` passing
