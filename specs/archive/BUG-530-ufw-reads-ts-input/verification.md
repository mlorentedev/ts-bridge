---
tags: [spec, verification, templates]
created: "2026-11-02"
---

# Verification - BUG-530-ufw-reads-ts-input

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

Measured at `b32458b` (#567 squashed as `7bd3fe7`, plus the review-gap fix). `features.json` holds one `go test -run` selector per criterion: 20, 9 and 4 passing subtests for f1–f3. The pre-merge figures (19/9/7 at `cd8fd10`) described a branch commit that the squash dropped from history.

- [x] AC1 -> `TestIptablesRule/ufw_host,_ts-input_accepts_tailscale0`, `TestSetupForLinux/ufw_host,...` (no `ufw allow`), `TestCheckForLinux/ufw_host,_ts-input...` (ok). Fixture: INPUT A verbatim.
- [x] AC2 -> `TestCheckForLinux/ufw_host,_two_leftover_copies_in_INPUT:_delete_both_(#530)` and `..._one_leftover_copy_beside_a_ufw_allow:_still_reported_(#530)`; the BUG-480 iptables-only rows are unchanged and pass.
- [x] AC3 -> `TestCheckForLinux/ufw_host,_iptables_unreadable:_unverified_(#530_review)` (the check-side row the review found missing), `TestIptablesRule/ufw_host,_iptables_unreadable`, `TestIptablesRule/ufw_host,_a_qualified_jump`, `TestSetupForLinux/ufw,_iptables_unreadable:_no_write_on_a_guess`, `TestCheckForLinux/ufw_host,_a_qualified_ts-input_jump:_unverified`.
- [x] AC4 -> the existing ufw allow/deny/absent rows, now also listing `lookpath iptables`, pass unchanged.
- [x] AC5 -> the mutants below, each run against `b32458b` and reverted.

### Mutants (AC5)

Counted as failing subtests (`--- FAIL: TestX/row`), not parent tests; the reviewer's figures (M1 22, M3 3) count differently on an earlier head.

| Mutant | Failing subtests at `b32458b` |
|---|---|
| M1: `readUFW` returns before the iptables read | 14 |
| M2: an unknown iptables answer is coerced to absent | 6 |
| M3: ufw copies use the iptables threshold (all but one) | 3 |
| M4: iptables is not credited when it decides (review gap) | 2: `TestSetupForLinux/ufw,_tailscaled's_ts-input_admits_the_tailnet:_no_ufw_allow_(#530)`, `TestCheckForLinux/ufw_host,_an_iptables_DROP_decides:_credited_to_iptables_(#530_review)` |

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

Default policy, recorded by the adversarial review (two `alpine:3.20` containers, ufw 0.36.2, iptables 1.8.10, `ufw allow 22/tcp`). The capture above used ufw's default `deny incoming`:

- Default deny: `iptables -S ufw-reject-input` is `-N ufw-reject-input` (empty), so a copy appended after ufw's jumps is reached and admits the port.
- `ufw default reject incoming`: `ufw-reject-input` holds `-A ufw-reject-input -j REJECT --reject-with icmp-port-unreachable`, which ends every packet before an appended copy, so the copy is inert.

Deleting every copy is right under either policy. The fix text now says the exposure holds under the default deny policy.

## Test status

- Test suite: `go test -race ./...` -> ok in every package; `golangci-lint run` -> 0 issues; `GOOS={linux,windows,darwin} go vet ./...` clean.
- Manual smoke test: none on a live ufw host; the fixtures are the captured output above.
- No regressions in existing test suite: yes.

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- A single copy on a ufw host is reported, not tolerated: setup never writes the iptables rule there, so any copy is a leftover that opens the port on every interface (owner decision, 2026-11-03).
- Review on #567: a failed `ufw show added` now still counts INPUT copies, and `host check` stays `unverified` with the copies and their delete command appended (`TestCheckForLinux/ufw_host,_ufw_unreadable,_a_leftover_copy:_still_reported_(#530)`, red on `cd8fd10`). Setup gained a ufw-host qualified-jump row. `features.json` f5 is narrowed: no command there re-runs the mutants.
- Adversarial review (`review.md`, PASS WITH GAPS), dispositioned in `b32458b`:
  - Messages now credit the tool whose rules decided: `(iptables)` for a `ts-input` allow or an iptables DROP on a ufw host. `backend` stays the tool setup writes through. New field `decidedBy`.
  - The copy fix text names its default-policy precondition.
  - The redundant `ufwErr == nil` guard is gone: a failed ufw read leaves `ruleUnknown`, which `== ruleAbsent` already excludes.
  - The check-side AC3 row was added.
  - Ticketed, not fixed here: the `features.json` vacuous-selector risk and the setup `ok` / check `manual` divergence on leftover copies (see the PR).
- `readUFW` was extracted from `readLinuxFirewall` to keep both under the complexity cap.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? no: the lesson (a front-end such as ufw hides rules written beneath it, so read the layer below before writing) is the BUG-492 one applied to ufw, and is already recorded there
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017's setup action is unchanged; scoping the write to the tailnet (option 1 in #530) would amend it and stays out of scope
- [x] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. no: host-firewall reading is specific to ts-bridge

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/BUG-530-ufw-reads-ts-input/` -> `specs/archive/BUG-530-ufw-reads-ts-input/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [x] Promotions above executed (if any)
