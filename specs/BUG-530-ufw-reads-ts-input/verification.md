---
tags: [spec, verification, templates]
created: "2026-11-02"
---

# Verification - BUG-530-ufw-reads-ts-input

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

All from commit `cd8fd10`; `features.json` holds one `go test -run` selector per criterion (19, 9 and 7 passing subtests for f1–f3).

- [x] AC1 -> `TestIptablesRule/ufw_host,_ts-input_accepts_tailscale0`, `TestSetupForLinux/ufw_host,...` (no `ufw allow`), `TestCheckForLinux/ufw_host,_ts-input...` (ok). Fixture: INPUT A verbatim.
- [x] AC2 -> `TestCheckForLinux/ufw_host,_two_leftover_copies_in_INPUT:_delete_both_(#530)` and `..._one_leftover_copy_beside_a_ufw_allow:_still_reported_(#530)`; the BUG-480 iptables-only rows are unchanged and pass.
- [x] AC3 -> `TestIptablesRule/ufw_host,_iptables_unreadable`, `TestIptablesRule/ufw_host,_a_qualified_jump`, `TestSetupForLinux/ufw,_iptables_unreadable:_no_write_on_a_guess`, `TestCheckForLinux/ufw_host,_a_qualified_ts-input_jump:_unverified`.
- [x] AC4 -> the existing ufw allow/deny/absent rows, now also listing `lookpath iptables`, pass unchanged.
- [x] AC5 -> the mutants below, each run against `cd8fd10` and reverted.

### Mutants (AC5)

| Mutant | Failing subtests |
|---|---|
| M1: `readUFW` returns before the iptables read | 19 |
| M2: an unknown iptables answer is coerced to absent | 4: `TestIptablesRule/ufw_host,_iptables_unreadable`, `TestIptablesRule/ufw_host,_a_qualified_jump`, `TestSetupForLinux/ufw,_iptables_unreadable:_no_write_on_a_guess`, `TestCheckForLinux/ufw_host,_a_qualified_ts-input_jump:_unverified` |
| M3: ufw copies use the iptables threshold (all but one) | 2: both `TestCheckForLinux/ufw_host,_..._leftover...(#530)` rows |

## Fixture capture

Captured on 2026-11-03 in `alpine:3.20` with `--cap-add NET_ADMIN`: ufw 0.36.2, iptables v1.8.10 (nf_tables). `ts-input` was replayed from BUG-492's recorded chain (`tsInputDefault`) and inserted with `iptables -I INPUT 1 -j ts-input`, as tailscaled does. The Ubuntu 24.04 image failed at `apt-get update` on this host (mirror signature error), so Alpine was used; the ufw and iptables versions are the same as in the existing Ubuntu fixtures.

A — ufw enabled with `ufw allow 22/tcp`, then tailscaled inserts its jump:

```
$ ufw show added
Added user rules (see 'ufw status' for running firewall):
ufw allow 22/tcp
$ iptables -S INPUT
-P INPUT DROP
-A INPUT -j ts-input
-A INPUT -j ufw-before-logging-input
-A INPUT -j ufw-before-input
-A INPUT -j ufw-after-input
-A INPUT -j ufw-after-logging-input
-A INPUT -j ufw-reject-input
-A INPUT -j ufw-track-input
$ iptables -S ts-input
-N ts-input
-A ts-input -s 100.64.0.5/32 -i lo -j ACCEPT
-A ts-input -i tailscale0 -j ACCEPT
-A ts-input -s 100.115.92.0/23 ! -i tailscale0 -j RETURN
-A ts-input -s 100.64.0.0/10 ! -i tailscale0 -j DROP
-A ts-input -p udp -m udp --dport 41641 -j ACCEPT
```

B — `ufw reload` after tailscaled: `iptables -S INPUT` is identical to A, and `-j ts-input` stays first.

C — two pre-#477 copies appended (`iptables -A INPUT -p tcp --dport 3389 -j ACCEPT`, twice):

```
$ iptables -S INPUT
-P INPUT DROP
-A INPUT -j ts-input
-A INPUT -j ufw-before-logging-input
-A INPUT -j ufw-before-input
-A INPUT -j ufw-after-input
-A INPUT -j ufw-after-logging-input
-A INPUT -j ufw-reject-input
-A INPUT -j ufw-track-input
-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT
-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT
```

## Test status

- Test suite: `go test -race ./...` -> ok in every package; `golangci-lint run` -> 0 issues; `GOOS={linux,windows,darwin} go vet ./...` clean.
- Manual smoke test: none on a live ufw host; the fixtures are the captured output above.
- No regressions in existing test suite: yes.

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- A single copy on a ufw host is reported, not tolerated: setup never writes the iptables rule there, so any copy is a leftover that opens the port on every interface (owner decision, 2026-11-03).
- `readUFW` was extracted from `readLinuxFirewall` to keep both under the complexity cap.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [ ] Lesson for the repo's `docs/lessons/`? <yes: path / no: reason>
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? <yes: path / no: reason>
- [ ] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. <yes: path / no: reason>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-530-ufw-reads-ts-input/` -> `specs/archive/BUG-530-ufw-reads-ts-input/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
