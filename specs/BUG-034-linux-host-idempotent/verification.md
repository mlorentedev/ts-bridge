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

## Review round 1 (nan/deepseek-v4-flash, FAIL at 79492ef)

- Major, REAL: `ufwAllows` read `ufw allow 3389/tcp comment 'xrdp'` and `ufw allow 3380:3390/tcp` as absent, so setup would add a duplicate. Applied: `ufwRuleMatchesPort` drops a trailing `comment` and matches the port inside a list or range (`portListHas`). `TestUFWAllows` gained the verbatim ufw 0.36.2 lines plus list, full-form range, a comment that names the port, and an app profile. Container proof (ubuntu:24.04, ufw 0.36.2): with `ufw allow 3389/tcp comment xrdp` already present, `host setup` reports "Firewall allows TCP 3389 (ufw)" and `ufw show added` still lists one rule.
- Minor, REAL (AC1 wording): applied. AC1 and the tasks line now say "runs no mutating command", the property `TestSetupForLinux` pins; the iptables read is a read-only `run`.
- Minor, REAL (`active` on an unreadable host): applied in `proposal.md`. Where `systemctl` cannot be read, `active` stays `null`, `enabled` false, and the step reports `unverified`; the sentence now says so.
- Minor, REAL (stale `checkFor` comment): applied in `internal/host/step.go`.
- Minor, REAL (`xrdp detection` fixture): applied in `cmd/cli/host_json_contract_test.go`.
- Minor, THEORETICAL (`LookPath` depends on `PATH`; `static`/`indirect` units): ticketed as #478.

## Review round 2 (nan/deepseek-v4-flash, FAIL at 854b799)

- Major, REAL (xrdp enabled but not running untested at step level): applied. `TestSetupForLinux` and `TestCheckForLinux` gained `xrdp enabled, not running` (`is-active` answers `failed`): setup runs `systemctl enable --now xrdp` and reports `changed`; check reports `changed` with `Active=false`. Mutating `x.enabled && x.active` to `x.enabled` turns both red.
- Minor, REAL (source-port ufw rule read as the destination): applied. `destPortIndex` takes the `port` after `to`; a `port` in the `from` clause never counts. Three `TestUFWAllows` cases, including the verbatim `ufw allow from any port 1234 to any port 3389 proto tcp`.
- Minor, REAL (proposal and tasks lag the parser): applied. The ufw sentence and the parser task line now name comments, lists, ranges and the source-port exclusion.
- Minor, REAL (iptables scoped allow reads as absent): out of scope here, recorded in the proposal and ticketed as #479.
- Minor, SPECULATIVE (deletion record): applied below.
- Minor, THEORETICAL (xrdp `LookPath`, `static` units): already #478.

## Test deletions

Five Linux cases left `internal/host/steps_test.go`; they guarded the pre-#454 behaviour this spec replaces.

| Removed case | Successor | Retained? |
|---|---|---|
| `TestSetupFor/linux with ufw` | `TestSetupForLinux/ufw, converged` | stricter: also pins no mutating command |
| `TestSetupFor/linux without ufw or xrdp, Tailscale down` | `TestSetupForLinux/xrdp missing, no systemctl, no backend, Tailscale down` | stricter: adds the no-backend branch |
| `TestCheckFor/linux, port found in ufw` | `TestCheckForLinux/converged` | equivalent, through `ufw show added` |
| `TestCheckFor/linux, firewall unreadable without root` | `TestCheckForLinux/not root, iptables host` | stricter: pins that no firewall command runs |
| `TestCheckFor/linux, nothing` | none on Linux | The all-zero legacy keys with Tailscale down are an OS-independent derivation (ADR-017 §3), asserted by `TestCheckFor/windows, registry unreadable and no rule`; the Linux reads it depended on are covered by `xrdp not installed` |

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
