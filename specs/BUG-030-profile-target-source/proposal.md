---
id: "BUG-030-profile-target-source"
type: spec
status: verifying
created: "2026-10-22"
issue: "mlorentedev/ts-bridge#417"
tags: [spec, proposal]
template_version: "1.0"
---

# BUG-030-profile-target-source

## Why

<!-- from issue #417: BUG-030: connect --profile help claims it overrides TS_TARGET, but env and YAML targets win -->

The profile flag's help and precedence tests were corrected in #442, but a
higher-priority target can still silently replace the selected profile's target.
An operator needs to see which source won before the bridge starts, without
changing the established precedence or inspecting credentials.

## What

After successful configuration validation, profile ownership and logger setup,
`connect` emits one structured warning when a non-empty effective TCP target
differs from the selected profile's non-empty target. It names the profile,
winning source (`--target`, `TS_TARGET` or `YAML`) and effective target.
`TS_TARGET` includes values loaded from the CWD `.env`.
The warning precedes the runner and remains visible with `--quiet`.

## Out of scope

- Changing flags > environment > YAML > profile > defaults, or the #399 model.
- Tracking every configuration field or distinguishing process environment from `.env`.
- Browser/host changes, real-network validation, credential mutations or release delivery.

## Risks / open questions

- Preserve the profile target from the existing read; a second read could race.
- Select the highest-priority target source, not a lower-priority conflicting value.
- Never log credentials, the configuration struct or an invalid target.
- Do not report a TCP target override when SOCKS5 mode clears that target.
- Resolved: use the existing structured logger after initialization, with the
  standard slog logger when the optional initialization hook is absent.

## Acceptance criteria

- [x] AC1: Flags, environment (including `.env`) and YAML retain existing target precedence; a changed profile target yields exactly one warning naming the winning source.
- [x] AC2: Profile-only, equal effective targets, no-profile use, SOCKS5 mode and invalid configurations produce no override warning.
- [x] AC3: Text and JSON warnings precede the runner, remain visible with `--quiet`, omit authentication material and leave the profile bytes unchanged.

## References

- Issue: [#417](https://github.com/mlorentedev/ts-bridge/issues/417).
- [ADR-012](../../docs/adr/adr-012-config-profiles-model.md): existing named-profile model.
- [Profile connection runbook](../../docs/runbooks/guide-profile-connections.md).
