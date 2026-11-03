---
tags: [spec, verification, templates]
created: "2026-11-02"
---

# Verification - BUG-530-ufw-reads-ts-input

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [ ] Criterion 1 -> commit `<hash>` / test `<name>`
- [ ] Criterion 2 -> commit `<hash>` / test `<name>`
- [ ] Criterion 3 -> commit `<hash>` / test `<name>`

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

- Test suite: `<command> -> <output / coverage %>`
- Manual smoke test: what was exercised, what was observed
- No regressions in existing test suite: yes / no (if no, document)

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

-
-

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
