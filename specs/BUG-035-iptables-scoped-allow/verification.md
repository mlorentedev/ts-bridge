---
tags: [spec, verification, templates]
created: "2026-10-22"
---

# Verification - BUG-035-iptables-scoped-allow

## Evidence

- [x] AC1 -> `TestIptablesCovers` (interface, source, `! -i`, multiport list and range, range, `--ports`, conntrack, comment), `TestSetupForLinux/iptables, converged` (a `-i tailscale0` allow: `ok`, no `-A`), `TestIptablesRule/present`
- [x] AC2 -> `TestSetupForLinux/iptables, rule missing` (only the `ts-input` jump: one `-A`, `changed`), `TestIptablesRule/absent behind a ts-input jump`
- [x] AC3 -> `TestIptablesCovers` (`33890`, udp, `--sport`, `! --dport`, `! --dports`, DROP, jump, no destination port, a comment spelling a covering rule, policy only)
- [x] AC4 -> `TestUFWAllows` (`from <net>`, `in on tailscale0`, `from <net> proto tcp` count; `proto udp`, `app OpenSSH`, a source port only do not)
- [x] AC5 -> `TestIptablesRule/permission`, `/no exit status`, `TestSetupForLinux/iptables -S fails` (`failed`, no `-A`); every iptables case records `output iptables -S INPUT` and `-A` is the only `run`

## Test status

- `go test -race -count=1 ./...` -> all packages ok
- `GOOS=linux|windows|darwin go vet ./...` -> clean; `golangci-lint run` -> 0 issues
- End to end in a privileged `ubuntu:24.04` container (iptables 1.8.10 nf_tables), `GOOS=linux` binary, no ufw:
  - `ts-input` chain with its `INPUT` jump plus `-A INPUT -i tailscale0 -p tcp --dport 3389 -j ACCEPT`: two `host setup` runs both report "Firewall allows TCP 3389 (iptables)", and `iptables -S INPUT` is unchanged (no unrestricted rule added).
  - Only the jump: the first run reports "Firewall rule added", the second "Firewall allows", and `-S INPUT` shows one `--dport 3389` rule.
- Measurements that shaped the parser (same image): `-S` prints `-p tcp -m tcp --dport`, puts `! ` before a negated flag, escapes quotes inside `--comment "..."`; ufw folds `from X to any` into `from X`.

## Decisions made during implementation

- Dropped two clauses from the first draft after review: "a jump in `INPUT` means unknown" (tailscaled adds `-j ts-input` on every host it manages, so setup would never act) and "a DROP before the ACCEPT means failed" (a scoped deny before a scoped allow is a legitimate hardened ruleset, and ufw never judged order). Shadowing is #481 for both backends.
- `--ports` (multiport, source or destination) counts: it matches the destination port too.
- `exitCode` is gone: `-S` either prints the rules or fails, so the exit status no longer carries an answer.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-043-2026-10-22.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: the ADR-017 Linux row (read first, then `iptables -A`) is unchanged; only what counts as present changed
- [x] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. no: single-project parser detail, captured in lesson 043

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-035-iptables-scoped-allow/` -> `specs/archive/BUG-035-iptables-scoped-allow/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
