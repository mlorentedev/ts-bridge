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

## Verification & Closing

- [x] [AC4] Run `go test -race ./...` and `golangci-lint run`.
- [x] Fill `verification.md`.
- [x] Open PR referencing #212 (`Closes #212`).
