---
tags: [spec, tasks, templates]
created: "2026-10-22"
---

# Tasks - BUG-030-profile-target-source

## Setup

- [x] Create external worktree `fix/issue-417-profile-target-source` from `origin/master`.
- [x] Verify the client/config baseline: `go test ./cmd/cli ./internal/config`.
- [x] Complete the owner-approved scope and acceptance criteria before code.
- [x] Check criteria-to-task coverage: AC1, AC2 and AC3 all map to the regression table and implementation below; no orphan tasks.

## Implementation

- [x] [AC1] [AC2] [AC3] Add `cmd/cli/connect_target_source_test.go` with table-driven command tests using temporary profiles, synthetic credentials and a captured logger.
- [x] [AC1] Run `go test ./cmd/cli -run TestConnectProfileTargetOverrideWarning -count=1`; expect failure because no override warning exists.
- [x] [AC1] [AC2] [AC3] Update `cmd/cli/connect.go` to preserve the profile target from its existing read and warn after validation, ownership and logger setup, before the runner.
- [x] [AC1] [AC2] [AC3] Re-run the regression command; expect pass for all source, no-warning and output-format cases.
- [x] [AC1] [AC2] [AC3] Run `go test ./cmd/cli ./internal/config/... -count=1` and `go vet ./...`; both pass.
- [ ] [AC1] [AC2] [AC3] Verify lint and Linux race tests through CI. Do not execute golangci-lint locally: the owner reports Cortex detection.
- [x] [AC1] [AC3] Explain source diagnostics and `.env` attribution in `docs/runbooks/guide-profile-connections.md` and capture the resolution-boundary lesson in `docs/lessons/lesson-045-profile-target-source.md`.

## Closing

- [x] Confirm every criterion has regression coverage and a `features.json` entry.
- [x] Fill `verification.md` with exact commands and results, including unavailable checks.
- [ ] Open an atomic PR with `Closes #417` and inspect all reviewer output.
- [ ] Obtain an independent review before archive; do not archive or merge in this implementation step.
