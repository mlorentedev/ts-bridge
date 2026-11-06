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
- [x] AC2 -> PR 2: unit tests on the iptables append and the printed form; e2e on a real VM with
  ufw removed, setup twice, `iptables -S INPUT` holds the scoped line, the second run is `ok`.
- [ ] AC3 -> PR 3: unit tests on the ufw write; e2e: `ufw show added` holds the scoped line.
- [ ] AC4 -> PR 2 (iptables, done) and PR 3 (ufw): unit tests per tier; e2e: a hand-added unscoped rule is reported by
  `check` and replaced by `setup`.
- [ ] AC5 -> PR 4: unit tests on the recreate command and the read. Windows e2e is pending.
- [ ] AC6 -> `scripts/e2e/linux.sh` (run), `scripts/e2e/windows.ps1` (written, run pending).

## Test status

PR 2 (Linux iptables):

- `go test -race ./...` exit 0. `golangci-lint run` (host, `GOOS=linux`, `GOOS=windows`) reports 0
  issues. `gosec ./...` exit 0.
- `make e2e-linux` on a throwaway Ubuntu 24.04 VM: 56 pass, 0 fail. The new cases remove ufw,
  then check: setup #1 `changed` and `iptables -S INPUT` holds exactly
  `-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT`; setup #2 `ok` (the printed form
  agrees with what setup writes); a hand-added `-A INPUT -p tcp --dport 3389 -j ACCEPT` is `changed`
  in check with `firewall_ok` false, replaced by setup, and `ok` after; a hand-added multiport allow
  is `manual` in check and left in place by setup.
- Mutants, each killed by the unit tests: no `-i` in the append (12 failures), never add the scoped
  rule (6), leave one legacy copy (6), count a scoped rule as legacy (12).

## Decisions made during implementation

See the proposal: two-tier replacement, Windows built-in group reported but not disabled, fixed `tailscale0`.

- PR 2 changes the copies machinery's meaning (#480, #530, #576): a "copy" is now a copy of the
  legacy unscoped rule. On an iptables host setup deletes all of them (before, one was setup's own
  rule and stayed); on a ufw host they stay a `manual` report until PR 3.
- A rule is "unscoped" when it names the port and carries no `-i`, `-s`, `-d` or `!`. A conntrack or
  comment qualifier is not read as scope, so such a rule is listed as foreign and reported rather
  than deleted.
- The e2e check for "ufw is gone" tests `/usr/sbin/ufw`, not `command -v`: the first VM run failed on
  `! command -v ufw` after the package was removed, most likely the shell's command hash for a binary
  it had run earlier (not confirmed), and the file test passed on the rerun.
