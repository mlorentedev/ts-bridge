---
tags: [spec, tasks]
created: "2026-11-06"
id: "SEC-631-scoped-firewall-rules"
type: spec-tasks
status: active
owner: manu
---

# Tasks - SEC-631-scoped-firewall-rules

Four atomic PRs, each at most 300 executable lines (tests excluded), stacked in this order. Only
the last says `Closes #631`.

## PR 1 - spec and ADR-017 Amendment B (docs only)

- [x] Branch from master: `fix/issue-631-scope-firewall-rules`
- [x] `proposal.md`, `tasks.md`, `verification.md`; forks decided in the proposal
- [x] [AC1] ADR-017 Amendment B, and the §5 table cells for Linux and Windows

## PR 2 - Linux iptables: scoped write and legacy replacement

- [x] [AC2] Failing test: the iptables append is `-A INPUT -i tailscale0 -p tcp --dport N -j ACCEPT`
  and its printed form equals what `iptables -S` prints (`-i` before `-p`)
- [x] [AC4] Failing tests: an unscoped port ACCEPT is legacy; exact copies are replaced (scoped
  added first, then `-D` once per copy); a foreign unscoped rule is `manual` and nothing is
  deleted; a failed add deletes nothing; a failed delete is `failed`
- [x] [AC4] `check`: legacy exact is `changed` (setup would act), foreign is `manual`, both
  `firewall_ok` false
- [x] The copies machinery (#480, #530, #576) follows the legacy form; on an iptables host every
  exact copy is now extra
- [x] [AC2] [AC4] `scripts/e2e/linux.sh`: iptables host (ufw removed) setup twice, `-S` prints the
  exact form, second run `ok`; a hand-added legacy rule is replaced
- [x] `features.json` for the Linux features; `docs/design/host-client-parity.md` rows updated (the Linux runbooks print no firewall command, so they need none)
- [x] `make e2e-linux` green (56 pass, 0 fail)

## PR 3 - Linux ufw: scoped write and legacy replacement

- [x] [AC3] Failing test: every ufw write is `allow in on tailscale0 to any port N proto tcp`, from
  one helper (`firewallSetupStep`, the inactive path and the fix strings)
- [x] [AC4] ufw legacy `allow N/tcp` is replaced (scoped first, then `ufw delete allow N/tcp`);
  other unscoped allows are `manual`; iptables copies on a ufw host are deleted only when ufw is
  active and a scoped rule is in place
- [x] The ufw inactive path writes the scoped allow and replaces ufw's legacy allow
- [x] `scripts/e2e/linux.sh`: the drift and cleanup cases use the scoped form; a hand-added
  `ufw allow 3389/tcp` is reported by check and replaced by setup
- [x] `make e2e-linux` green (56 pass, 0 fail)

## PR 4 - Windows

- [ ] [AC5] Failing test: the recreate command carries `-RemoteAddress
  100.64.0.0/10,fd7a:115c:a1e0::/48` and no `Enable-NetFirewallRule`
- [ ] [AC5] `windowsFirewallRuleOK` compares `RemoteAddress` as a set; `Any` is a legacy rule, and
  check says so
- [ ] [AC5] Failing test: the recreate creates the scoped rule before it removes the old ones,
  and a failed create is terminating (no remove runs)
- [ ] [AC5] Failing test: `check` and `setup` report an enabled built-in `Remote Desktop` group as
  `manual` with the disable command, and `firewall_ok` is false (folds in #685)
- [ ] [AC6] `scripts/e2e/windows.ps1` cases written; the run is pending (the guest is shared)
- [ ] `features.json`, the Windows runbook, `Closes #631` and `Closes #685`, each on its own line

## Closing (every PR)

- [ ] `go test -race ./...`, `golangci-lint run` (host, `GOOS=linux`, `GOOS=windows`), `gosec ./...`
- [ ] `verification.md` filled with the command output
