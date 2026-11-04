---
tags: [spec, tasks]
created: "2026-11-04"
id: "HOST-455-windows-host-state"
type: spec-tasks
status: active
owner: manu
---

# Tasks - HOST-455-windows-host-state

> TDD order. Three PRs under one spec, in order W1 → W2 → W3; each lands on its own.

## Setup

- [x] Branch created from master: `fix/issue-455-windows-host-state`
- [x] `proposal.md` complete, acceptance criteria testable
- [x] Open questions resolved in `proposal.md` (debug-prefs drift, display-group call, read errors)

## PR 1 — W1 unattended mode (this spec lands here)

- [x] [AC1] Failing table test over four prefs fixtures; in none of them is any call `tailscale up`:
  - `ForceDaemon:true` → ok, no `set`;
  - Headscale prefs with `ForceDaemon:false` → `set --unattended`, changed;
  - unparseable prefs → `set`, changed;
  - `set` fails → failed, with a fix.
- [x] [AC1] Implement `unattendedStep` reading `tailscale debug prefs`
- [x] [AC4] Update `windowsSetupCalls` and the existing Windows setup tables

## PR 2 — W2 effective RDP state

- [x] [AC2] Failing table test over (policy, local), for setup and check:
  - (absent, 0) → ok;
  - (absent, 1) → changed, with a write;
  - (absent, absent) → changed, with a write;
  - (1, 0) → manual, no write;
  - (0, 1) → ok;
  - read error → failed.
- [x] [AC2] Implement the deny read, `effectiveDeny`, and the setup/check mapping
- [x] [AC2] Mutation check: force `effectiveDeny` to ignore the policy value → a test fails

## PR 3 — W3 firewall rule

- [ ] [AC3] Failing table test, for setup and check:
  - rule absent;
  - present, same port, enabled → ok, no Remove/New;
  - on another port;
  - disabled;
  - duplicated;
  - read error.
- [ ] [AC3] Implement `readWindowsFirewallRule` and act only on drift
- [ ] [AC3] Mutation check: a wrong-port read must make setup act

## Closing

- [ ] Every AC covered by a named test; every `features.json` entry has a real command
- [ ] `go test -race ./...`, `golangci-lint run`, `gosec ./...` green; `GOOS=windows go vet ./...`
- [ ] Parity doc rows W1-W3 updated; `verification.md` filled
