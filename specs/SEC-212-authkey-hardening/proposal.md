---
id: "SEC-212-authkey-hardening"
type: spec
status: draft
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
   - Update deployment runbooks (`docs/runbooks/guide-deployment-windows.md`, `docs/runbooks/guide-deployment-linux.md`, `docs/runbooks/guide-multi-device-operations.md`, `docs/troubleshooting/security-audit.md`) to use `--auth-key-file` in command examples instead of inline `--auth-key`.
4. **Tests (`cmd/cli/init_test.go`, etc.)**:
   - Ensure CLI tests verify updated help text, next-steps output, and error messages.

## Out of Scope

- Removing `TS_AUTHKEY` or `--auth-key` support (backward compatibility is preserved).
- Changing the precedence order in `Merge()` (flags > env > YAML > defaults remains unchanged).

## Acceptance Criteria

- [AC1] `README.md` and `.env.example` highlight `--auth-key-file` as the recommended method with explicit risk model explanations.
- [AC2] `cmd/cli/init.go` (help text, long description, next steps) promotes `--auth-key-file` and warns of process list / environment inheritance.
- [AC3] Documentation and deployment runbooks under `docs/` use `--auth-key-file` in CLI examples.
- [AC4] All unit tests in `cmd/cli/` and across the repository pass without regressions.
