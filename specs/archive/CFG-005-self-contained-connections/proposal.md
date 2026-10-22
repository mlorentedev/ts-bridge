---
id: "CFG-005-self-contained-connections"
type: spec
status: archived
created: "2026-10-15"
issue: "mlorentedev/ts-bridge#441"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# CFG-005-self-contained-connections

## Why

<!-- from issue #441: CFG-005: Self-contained daily profile connections -->

An existing Windows/Git Bash profile and managed Headscale credential still need a hand-maintained launcher selecting a second YAML file. Repeating connection parameters outside the profile makes daily operation fragile and obscures SSH authentication and process ownership. Deliver one foreground `connect --profile <name>` operation without managing SSH secrets or unrelated profiles.

## What

Extend local profiles with non-secret runtime settings: bootstrap SSH endpoint and SOCKS address, hostname, state directory, local bind address, and health address. One-time `init --profile <name> --config <yaml> --force` imports these settings while preserving the existing target and managed credential when omitted. Existing flag/env/YAML overrides keep their precedence.

Acquire a per-profile OS-backed ownership lock before logging/runtime startup. A duplicate fails with remediation; normal exit or parent-process death releases ownership automatically. Never identify ownership from a listening port, kill another process, or add background reuse.

Continue using system OpenSSH and inherited terminal streams for interactive authentication. Report the selected executable and actionable identity/agent guidance after startup failure without capturing secrets. Verify destination-service connectivity separately from bridge readiness.

## Out of scope

- The broader #399 env migration/deprecation, provider-backed credential rotation, or credential-store transactions.
- Automatic SSH-agent/service installation, key/passphrase handling, background supervision, or stopping unrelated processes.
- DNS, hosts-file, system proxy, SSH/TLS verification weakening, or Gitea tokens.

## Risks / open questions

- Resolved: preserve existing precedence; #417 remains the broader source-diagnostics work. Correct misleading help when touching this surface.
- Resolved: use OS-backed file locks, not PID files or hash-selected TCP ports. Persistent lock files contain no secrets and are not removed during normal release.
- Resolved: runtime state paths are machine-local non-secret settings; credential file paths and secret values remain prohibited. Shareable descriptors do not export runtime settings.
- Boundary: abrupt parent termination can leave an SSH child alive (#376). A subsequent attempt must fail on the occupied bootstrap address, not reuse or kill that child.
- Live validation requires the operator's interactive SSH unlock if authentication prompts. No unit-test result substitutes for that evidence.
- Delivery limit: stop and split if executable production changes exceed 300 lines.

## Acceptance criteria

- [x] AC1: Local profiles persist runtime settings across reads, target updates, descriptor imports, and credential association changes; legacy profiles remain loadable and descriptors remain unchanged.
- [x] AC2: One-time supported profile configuration preserves the existing target/credential and makes `connect --profile <name>` reach the runner with the runtime settings, without a separate config argument. Explicit overrides still win; browser profile handling does not inherit a TCP target.
- [x] AC3: Repeated/concurrent startup of one named profile fails before a second runner/SSH child; independent profiles remain usable, and ownership is released on exit, including abrupt lock-owner process exit.
- [x] AC4: Missing or failing OpenSSH provides actionable client/identity/agent diagnostics without mandatory agent ownership, secret disclosure, or disabling interactive unlock.
- [ ] AC5: The Windows/Git Bash daily command is exercised against the real destination service with normal TLS verification and no application token; duplicate startup and unrelated-profile preservation are checked. The runbook records the exact operational contract.

## References

- #441, #399, #417, #376, #183.
- `docs/adr/adr-016-self-contained-profile-connections.md`.
- ADR-005, ADR-010, ADR-012, ADR-013, ADR-015.
