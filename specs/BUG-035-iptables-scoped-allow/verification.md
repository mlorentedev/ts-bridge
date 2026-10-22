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

## Review round 1 (nan/deepseek-v4-flash, FAIL at ef191e4)

- Major, REAL (the comment strip had no test that fails without it): applied. `TestIptablesCovers/comment holding -j ACCEPT` (a covering rule whose comment contains `-j ACCEPT`, want present) fails when the strip is replaced by `strings.Fields(line)` and passes with it. The old case is renamed `comment port never counts`, which is what it pins.
- Major, THEORETICAL (a blanket iptables ACCEPT without a port reads absent while ufw's reads present): ticketed as #483. The "no destination port" exclusion protects against `-m conntrack` rules, and narrowing it needs a contract decision.
- Minor, REAL (no check-side iptables cases for AC1/AC5): applied. `TestCheckForLinux/iptables, scoped allow` (all `ok`, `firewall_ok`) and `/iptables -S fails` (`unverified`, no `run`).
- Minor, REAL (stale `-C` comments): applied. The `TestIptablesRule` and `TestCheckForLinux` doc comments now describe the `-S` read.
- Minor, THEORETICAL (`-d` destination-qualified ACCEPT counts): declined. A rule the operator scoped by destination address is still their scope, which the proposal says check does not judge. Message accuracy for scoped rules goes with #481.
- Question, SPECULATIVE (repeated `--dport` in one rule; `ufw proto any`): declined. Neither form was produced by the tools measured, and both read absent, which leads to one extra narrow rule rather than a widening.

## Review round 2 (nan/deepseek-v4-flash, PASS-WITH-GAPS at 8733c9b)

- Major, THEORETICAL (blanket ACCEPT without a port reads absent): carried; already #483.
- Minor, REAL (`checkFor` doc comment in `internal/host/step.go` still named `-C`): applied. Round 1's comment finding had fixed only the two test comments; this was the third.
- Minor, THEORETICAL (`-d` destination-qualified allow reads present): declined, as in round 1; same on both backends, and setup adds nothing. Revisit under #481.
- Question, SPECULATIVE (repeated `--dport`; `-m limit` ACCEPT reads present): declined; neither leads to a widening, and neither form was produced by the tools measured.

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
