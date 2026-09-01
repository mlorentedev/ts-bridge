---
id: "SEC-212-authkey-hardening"
type: spec
status: draft
created: "2026-08-14"
issue: "ts-bridge#212"
tags: [spec, tasks, security, hardening]
template_version: "1.0"
---

# Tasks - SEC-212: Auth Key Security Hardening

> TDD order. One task = one focused commit. Tick as you go.

## Setup

- [x] Spec created in `specs/SEC-212-authkey-hardening/`
- [x] Gating issue verified: #212

## Implementation

- [x] [AC2] [AC4] Update `cmd/cli/init.go` and `cmd/cli/init_test.go` to promote `--auth-key-file`, document process list and child-process visibility warnings, and update `printNextSteps`.
- [x] [AC1] Update `README.md` and `.env.example` to prominently feature `--auth-key-file` and document security risks of plaintext keys.
- [x] [AC3] Update `docs/runbooks/` and `docs/troubleshooting/` examples to recommend `--auth-key-file` instead of bare `--auth-key`.
  - Delivered by PR #298: `docs/runbooks/guide-deployment-windows.md`, `docs/troubleshooting/security-audit.md`, `cmd/cli/init.go`, `.env.example`.
  - Delivered here: `README.md` minimal-setup line, `docs/runbooks/guide-multi-device-operations.md` (masked `init` prompt + `connect --auth-key-file` in Launch Commands).
  - **Not** delivered: `docs/runbooks/guide-deployment-linux.md`. Its systemd path authenticates through `EnvironmentFile`, and `--auth-key-file` is a `connect` flag with no unit in-tree to pass it — the unit the runbook copies was deleted in #156. Promoting the key file there needs the unit back first, so it is deferred to #307 rather than ticked again on an incomplete diff.

## Verification & Closing

- [x] [AC4] Run `go test -race ./...` and `golangci-lint run`.
- [x] Fill `verification.md`.
- [x] Open PR referencing #212 (`Closes #212`).

## Follow-up (issue #305)

- [x] Recover the uncommitted remainder of AC1/AC3 left behind by PR #298 (`README.md`,
      multi-device runbook) and land it.
- [x] Correct the AC3 file paths in `proposal.md` to be repo-relative.
- [x] Verify every documented flag against the built binary — this found that `init` has no
      `--auth-key-file` (only `connect` registers it), so the runbook must not show it there.
      Gap filed as #306.
