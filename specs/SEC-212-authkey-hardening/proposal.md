---
id: "SEC-212-authkey-hardening"
type: spec
status: verifying # draft | implementing | verifying | archived; implementation merged in #298 and #310
created: "2026-08-14"
issue: "ts-bridge#212"
tags: [spec, security, hardening, auth-key, docs, cli]
template_version: "1.0"
---

# SEC-212: Discourage Plaintext TS_AUTHKEY in env/.env; Promote --auth-key-file

<!-- from issue #212: Docs/hardening: discourage plaintext TS_AUTHKEY in env/.env; promote --auth-key-file in examples and init -->

## Why

The default and example setup has historically shown `TS_AUTHKEY` directly in a `.env` file or via `--auth-key` flag. However:
1. Environment variables (`TS_AUTHKEY`) are inherited by and visible to child processes.
2. The `--auth-key` CLI flag is visible in the operating system's process table (`ps`, Task Manager).
3. `--auth-key-file` already exists as the secure alternative (reading the key from a restricted `0600` file), but was treated as secondary in documentation and CLI examples.

## What

Harden documentation, examples, and the `init` wizard to establish `--auth-key-file` as the recommended security best practice:

1. **CLI `init` wizard & command guidance (`cmd/cli/init.go`)**:
   - Update `init` help text and examples to emphasize `--auth-key-file`.
   - Update `printNextSteps` in `init` to highlight `--auth-key-file` usage alongside `.env`.
   - Clarify security notes regarding process environment inheritance and process list exposure.
2. **README and `.env.example` (`README.md`, `.env.example`)**:
   - Feature `--auth-key-file` prominently in Quick Start, examples, and configuration tables.
   - Explicitly document the risk model (process environment vs. process list vs. dedicated `0600` key file).
3. **Runbooks and Documentation (`docs/`)**:
   - Update deployment runbooks (`docs/runbooks/guide-deployment-windows.md`, `docs/runbooks/guide-multi-device-operations.md`, `docs/troubleshooting/security-audit.md`) to use `--auth-key-file` in command examples instead of inline `--auth-key`.
   - `docs/runbooks/guide-deployment-linux.md` was in this list originally and is now out of scope (see below): it has no CLI example to convert.
4. **Tests (`cmd/cli/init_test.go`, etc.)**:
   - Ensure CLI tests verify updated help text, next-steps output, and error messages.

## Out of Scope

- Removing `TS_AUTHKEY` or `--auth-key` support (backward compatibility is preserved).
- Changing the precedence order in `Merge()` (flags > env > YAML > defaults remains unchanged).
- The Linux systemd runbook (`docs/runbooks/guide-deployment-linux.md`) → **#307**. It
  authenticates through a `0600` `EnvironmentFile` and has no CLI auth-key example. Moving it to
  `--auth-key-file` needs an `ExecStart` that carries the flag (there is no `TS_AUTHKEY_FILE`
  env var), and the unit it copies was deleted in #156. Restoring that unit is its own change.
- A key-file flag on `init` → **#306**. Only `connect` registers `--auth-key-file`, so
  unattended `init` examples still pass `--auth-key` and are labelled as visible in the process
  table.

## Acceptance Criteria

- [AC1] `README.md` and `.env.example` highlight `--auth-key-file` as the recommended method with explicit risk model explanations.
- [AC2] `cmd/cli/init.go` (help text, long description, next steps) promotes `--auth-key-file` and warns of process list / environment inheritance.
- [AC3] Every `connect` CLI example in documentation and runbooks under `docs/` supplies the key with `--auth-key-file`, never inline `--auth-key`. The only inline `--auth-key` examples left are unattended `init` calls, which cannot take a key file until #306, and each one carries a process-table warning. The Linux systemd runbook is out of scope (#307).
- [AC4] All unit tests in `cmd/cli/` and across the repository pass without regressions.
