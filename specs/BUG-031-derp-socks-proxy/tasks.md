---
tags: [spec, tasks, templates]
created: "2026-10-19"
---

# Tasks - BUG-031-derp-socks-proxy

## Setup

- [x] Open #443, P1/bug, self-assigned and In Progress.
- [x] External `fix/derp-socks-proxy` worktree from `origin/master`, no gitlinks.
- [x] Clean baseline: `go test .\internal\bootstrapssh .\cmd\cli` passes.
- [x] Complete proposal and map all four criteria to tasks/features before code.

## Implementation

- [x] [AC1] Add `internal/bootstrapssh/control_proxy_test.go` with isolated
  actual SDK control/DERP consumers; run `go test .\internal\bootstrapssh
  -run TestControlProxySDKConsumers -count=1 -v`, expecting DERP CONNECT failure.
- [x] [AC1] Implement loopback protocol adapter in `control_proxy.go`, update
  the selector and `cmd/cli/run.go` context wiring; rerun the regression to green.
- [x] [AC2] Add negative host/failure and ordinary HTTP cases first, then make
  `go test .\internal\bootstrapssh -run TestControlProxy -count=1` pass.
- [x] [AC3] Add cancellation/active relay and startup cleanup cases first,
  then make the same focused command pass.
- [x] [AC4] Run `go test .\internal\bootstrapssh .\cmd\cli`, build the candidate,
  amend ADR-005 and the SSH-bootstrap runbook with the protocol contract.
- [x] [AC4] Correct the readiness-timeout test's measurement boundary without
  changing production timeouts; reproduce the latest zero-duration assertion
  before choosing the test repair. Rerun repeated regression and full suite.
- [x] [AC4] Live Windows DERP acceptance is deferred to #441 and is not claimed
  here; Gitea is not claimed to pass.

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` (see below) with a non-vacuous verification command
- [x] Type checks pass
- [ ] Lint passes (local golangci-lint is built with an older Go; check off from the PR's CI `lint` job)
- [x] No unrelated changes in the diff (no scope creep)
- [x] `verification.md` filled in
- [x] Capture the SDK-consumer protocol lesson in the repository lesson index.
- [ ] PR opened referencing this spec folder
