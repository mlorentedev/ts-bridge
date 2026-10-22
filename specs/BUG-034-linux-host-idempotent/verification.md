---
tags: [spec, verification, templates]
created: "2026-10-22"
---

# Verification - BUG-034-linux-host-idempotent

## Evidence

- [x] AC1 (read first, a second run changes nothing) -> `TestSetupForLinux/ufw,_converged` and `iptables,_converged` record no `run` call except the read-only `-C`. In the container, two real `host setup` runs left exactly one rule on each backend.
- [x] AC2 (ufw `allow`, iptables `-C` before `-A`) -> `TestSetupForLinux/ufw,_nothing_configured` and `iptables,_rule_missing` (golden argv); `TestIptablesRule`.
- [x] AC3 (exact port, inbound only) -> `TestUFWAllows`: `33890`, `338`, `udp`, `out`, `route`, `deny` and `limit` do not match; `3389/tcp`, `3389`, `to any port 3389` and `in on tailscale0 ... port 3389 proto tcp` do.
- [x] AC4 (xrdp through `systemctl`) -> `TestXRDP` (enabled+active, disabled+inactive, enabled+failed, not installed, no systemctl, empty answer); `TestSetupForLinux` and `TestCheckForLinux`.
- [x] AC5 (unreadable means `unverified` or `failed`, with no mutation) -> `TestSetupForLinux/not_root:_no_firewall_command_runs`, `iptables_-C_fails_with_another_status`, `no_systemctl`; `TestCheckForLinux/not_root,_iptables_host` and `no_systemctl,_ufw_unreadable`; `TestCheckForLinux` asserts that check never runs a mutating command.

## Test status

- `go test -race -count=1 ./...` -> ok
- `GOOS=linux|windows|darwin go vet ./...` -> clean
- `golangci-lint run` and `GOOS=linux golangci-lint run ./internal/host/...` -> 0 issues
- End to end in an `ubuntu:24.04` container (ufw 0.36.2, iptables 1.8.10 nf_tables), with the `GOOS=linux` binary:
  - ufw: first setup reports "Firewall rule added: TCP 3389 (ufw)", the second "Firewall allows TCP 3389 (ufw)", and `ufw show added` lists one rule.
  - Removing ufw switches to iptables: same two messages, and `iptables -S INPUT` shows one `--dport 3389` rule.
  - `host check --json` as root reports the firewall `ok` (`firewall_ok: true`). As a non-root user it reports `Firewall configuration unverified | run ts-bridge host check as root`.
- No regressions: the Windows `TestSetupFor` / `TestCheckFor` cases pass unchanged. The fake runner only gained `exit` and `elevated`.

## Decisions made during implementation

- Elevation is part of the `Runner` (`Elevated()`), so the gate is table-tested like any other input (lesson 040). It is checked before any firewall command, because legacy iptables may exit 1, the "absent" answer, on permission denied.
- ufw is read with `ufw show added`, not `ufw status`, because `status` lists no rules while ufw is inactive. The container run showed that.
- An interface-scoped allow (`in on tailscale0`) counts as allowing the port. For a Tailscale host it is the narrower and better rule.
- `cfg.FirewallRule` no longer appears in the Linux messages. Neither ufw nor iptables names rules, so the old message named a rule that does not exist.
- The step is renamed from `xrdp detection` to `xrdp service`, since it now reads the service and not the binary. `steps[].name` is not a key consumers match on (ADR-017 §3 pins `outcome`), so the rename is not a contract break.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-042-2026-10-22.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017 (per-OS table, Linux row) already decides ufw `allow`, iptables `-C` then `-A`, and `systemctl enable --now`
- [x] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. no: single-project mechanism (one CLI's Linux host steps)

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-034-linux-host-idempotent/` -> `specs/archive/BUG-034-linux-host-idempotent/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
