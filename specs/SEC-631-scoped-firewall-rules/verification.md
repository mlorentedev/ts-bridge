---
tags: [spec, verification]
created: "2026-11-06"
id: "SEC-631-scoped-firewall-rules"
type: spec-verification
status: active
owner: manu
---

# Verification - SEC-631-scoped-firewall-rules

## Evidence

- [ ] AC1 -> `docs/adr/adr-017-host-desired-state.md`, Amendment B (PR 1).
- [ ] AC2 -> PR 2: unit tests on the iptables append and the printed form; e2e on a real VM with
  ufw removed, setup twice, `iptables -S INPUT` holds the scoped line, the second run is `ok`.
- [ ] AC3 -> PR 3: unit tests on the ufw write; e2e: `ufw show added` holds the scoped line.
- [ ] AC4 -> PR 2 and PR 3: unit tests per tier; e2e: a hand-added unscoped rule is reported by
  `check` and replaced by `setup`.
- [ ] AC5 -> PR 4: unit tests on the recreate command and the read. Windows e2e is pending.
- [ ] AC6 -> `scripts/e2e/linux.sh` (run), `scripts/e2e/windows.ps1` (written, run pending).

## Test status

Filled per PR with the command output.

## Decisions made during implementation

See the proposal: two-tier replacement, Windows built-in group left as is, fixed `tailscale0`.
