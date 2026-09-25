---
id: "CLI-306-init-auth-key-file"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-09-23"
issue: "mlorentedev/ts-bridge#306"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# CLI-306-init-auth-key-file

> **Naming**: file lives at `<repo>/specs/CLI-306-init-auth-key-file/proposal.md`. `CLI-306-init-auth-key-file` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

<!-- from issue #306: feat(cli): init has no --auth-key-file, so non-interactive provisioning cannot avoid the process table -->

`ts-bridge init` can provision a configuration interactively without exposing the auth key, but unattended provisioning currently requires `--auth-key`, which exposes the credential in the process table. This makes the documented secure key-file workflow inconsistent across commands and blocks safe automation on shared or corporate Windows machines. The work is tracked by issue #306.

## What

`ts-bridge init --auth-key-file <path> --target <host:port>` reads the auth key from the file and creates the requested configuration without placing the credential in the command line. The file value takes precedence over `--auth-key`, while an explicitly supplied `--auth-key` still produces the existing process-table warning. Missing, empty, or malformed key files return actionable errors.

## Out of scope

- Package-manager distribution through WinGet, Scoop, Chocolatey, or an installer.
- Changes to `connect`, `discover`, profile storage, auth-key formats, or configuration precedence outside `init`.
- Real-tailnet connectivity behavior or the hardware E2E scope tracked by #183.

## Risks / open questions

- Reuse the existing `readAuthKeyFile` helper so permission checks, newline trimming, and error wording do not drift between commands.
- `--auth-key-file` is incompatible with `init --profile`, matching the existing rule that profiles never store auth keys.
- No blocking open questions remain; the issue explicitly selects file-over-inline precedence.

## Acceptance criteria

Observable outcomes. Each must be testable.

- [x] `init --auth-key-file <file> --target <host:port>` writes a valid config using the file value, and missing, empty, or malformed files fail with clear errors.
- [x] `--auth-key-file` takes precedence over `--auth-key`; when both are passed, the inline-key process-table warning is still emitted.
- [x] `init --help`, README, the multi-device runbook, and the generated documentation describe the secure non-interactive key-file workflow without claiming that `init` lacks the flag.
- [x] Native Go tests exercise the production command tree on Windows and Linux; the full Go test suite, vet, lint, and build remain green.

## References

- Bitácora board: the GitHub issue / Project item tracking this spec (see the `issue:` frontmatter field)
- Related ADR: `docs/adr/adr-013-cli-tests-in-go.md`
- Related patterns: `00_meta/patterns/pattern-testing-standards.md`
