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

- [x] AC1 -> `docs/adr/adr-017-host-desired-state.md`, Amendment B (PR 1).
- [x] AC2 -> PR 2: unit tests on the iptables append and the printed form; e2e on a real VM with
  ufw removed, setup twice, `iptables -S INPUT` holds the scoped line, the second run is `ok`.
- [x] AC3 -> PR 3: unit tests on the ufw write; e2e: `ufw show added` holds the scoped line.
- [x] AC4 -> PR 2 (iptables) and PR 3 (ufw): unit tests per tier; e2e: a hand-added unscoped rule is reported by
  `check` and replaced by `setup`.
- [x] AC5 -> PR 4: unit tests on the recreate command and the read; Windows e2e run (see PR 4 below).
- [x] AC6 -> `scripts/e2e/linux.sh` (run), `scripts/e2e/windows.ps1` (run).
- [x] AC7 -> PR 5: `TestOffTailnetOnlyAllow` (iptables and ufw, a source and an interface), a
  `TestCheckForLinux` row (`firewall_ok` false); e2e on both backends (see PR 5 below).

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

PR 3 (Linux ufw):

- `go test -race ./...` exit 0. `golangci-lint run` (host, `GOOS=linux`, `GOOS=windows`) 0 issues.
  `gosec ./...` exit 0.
- `make e2e-linux` on a throwaway Ubuntu 24.04 VM: 77 pass, 0 fail, at 66f28d6 (after the merge from master). New cases: setup writes the allow on tailscale0 and
  no bare 3389 allow beside it; a hand-added `ufw allow 3389/tcp` is `changed` in check with
  `firewall_ok` false, replaced by setup, `ok` after; a foreign `ufw allow 22,3389/tcp` is `manual`
  and left in place; a legacy iptables copy on a ufw host is `changed`, then deleted by setup.
- Mutants, each killed by the unit tests: wrong interface in the ufw write (16 failures), legacy ufw
  allow ignored (13), scoped rule never added (11), scoped ts-input rule ignored (3), delete any
  unscoped allow (9). An allow off the tailnet (another interface, a source or a destination) is not
  the scoped rule, so setup still adds the tailscale0 allow before it deletes a legacy one; each of
  the three conditions in `ufwAllowOffTailnet`, removed, fails `TestReadUFWUnscoped`.

PR 4 (Windows):

- `go test -race ./...`, `golangci-lint run` (host, `GOOS=linux`, `GOOS=windows`) and `gosec ./...`:
  see the PR body.
- Mutants, each killed by the unit tests: no `-RemoteAddress` in the recreate (19 failures), any
  remote accepted (5), IPv4 mask not normalized (2), IPv6 case not folded (2), legacy never
  reported (2), recreate errors not terminating (19), remove before create (19), remove by display name (1), check ignores the
  built-in group (5), group never reported (3).
- `make e2e-windows` on a throwaway Windows 11 Arm64 guest, at 310bfc6 (after the merge from master):
  50 pass, 0 fail. The #631 cases: the rule is scoped to the tailnet addresses, as Windows prints
  them, so the read matches the real `RemoteAddress` form; an enabled built-in Remote Desktop group
  is reported by check and left on by setup; once the operator disables it the firewall is ok
  again; a legacy rule open to any address is seen by check and repaired by setup.

PR 5 (review round 1 fixes):

- `go test -race ./...` exit 0; `golangci-lint run` 0 issues; `GOOS=windows go vet ./internal/host` clean.
- Red first: `TestOffTailnetOnlyAllow` failed 4/4 (setup `ok` with no write, check `ok`) before
  `offTailnetOnly`; the two `TestWindowsBuiltinGroup` rows failed (`ok`, want `unverified`) before
  the parse check.
- `make e2e-linux` at 12ea1b5 (code as at 27ccaf9) on a throwaway Ubuntu 24.04 VM: 84 pass, 0 fail
  (`linux-20261110T060947Z.log`). The new cases add an allow scoped to `10.0.0.0/8` on ufw and on
  iptables: check says `changed` (`firewall_ok` false), setup adds the `tailscale0` rule and keeps
  the operator's, check is `ok` afterwards.
- `make e2e-windows` at 27ccaf9 on a throwaway Windows 11 Arm64 guest: 52 pass, 0 fail
  (`windows-20261110T060817Z.log`). On the fresh guest the built-in Remote Desktop group is
  disabled, and it is still disabled after setup.

## Review disposition (round 1, FAIL)

`review.md` (nan/deepseek-v4-flash, at 55cfd68) returned FAIL on one REAL Major. Each finding:

- An allow scoped only off the tailnet reads as converged (Major, REAL): applied (PR 5, AC7).
  `offTailnetOnly` sends setup to add the `tailscale0` rule and check to report `changed`, and
  `firewall_ok` is false. The operator's rule stays. An address scope counts as off the tailnet
  even when it is `100.64.0.0/10`; the proposal records why.
- The built-in group is enabled by default on Windows (Major, THEORETICAL): refuted on a real host.
  The fresh guest has the group disabled, and setup leaves it disabled. Two e2e cases pin this, so
  a guest image that ships it enabled turns the run red.
- Spec files disagree on whether the Windows e2e ran (Minor, REAL): applied. The proposal Risks,
  AC6 and `features.json` f12 now agree with `tasks.md` and this file.
- Closing boxes unticked; no Promotion candidates (Minor, REAL): applied. `tasks.md` Closing is
  ticked, and this file has the section below.
- A garbled `exposed=` reads as clean (Minor, SPECULATIVE): applied anyway, because it is cheap and
  fails closed. A count that does not parse is `unverified`, the same as a failed read, and two
  `TestWindowsBuiltinGroup` rows pin it.
- An unscoped accept inside ts-input counts as scoped (Minor, SPECULATIVE): recorded. A comment on
  `iptUnscopedRead` states the assumption: tailscaled writes no such rule (the captures in #526
  show only `-i tailscale0` and loopback), and the chain is tailscaled's to rewrite.

## Review disposition (round 2, FAIL)

Reviewer `nan/mimo-v2.6-flash`, at e6fbbcb (`review.md`; round 1 kept as `review-round1.md`). Fixed in f08036a.

- **Major REAL: an inactive ufw whose only allow is scoped off the tailnet.** Applied. `ufwInactiveState` now checks `offTailnetOnly` before `ufwAllows`. Setup writes `ufw allow in on tailscale0 ...` for when ufw is enabled, and both commands say that the existing allow admits only addresses or interfaces off the tailnet.
  - After the legacy-allow replacement the read is marked scoped. That keeps "a legacy allow is replaced" saying "allow is in place".
  - New `TestUFWInactive` row "an allow scoped off the tailnet" (setup and check): red before the fix.
  - New e2e case "ufw inactive, off-tailnet allow only" in `scripts/e2e/linux.sh`. `make e2e-linux` at f08036a: 85 pass, 0 fail, including this case (`scripts/e2e/out/linux-20261111T024834Z.log`). The Windows change is message-only and is pinned by unit tests, so Windows e2e was not rerun.
- **Major THEORETICAL: a negated scope (`! -i tailscale0 ... -j ACCEPT`) passed as the scoped rule.** Applied anyway, since the fix is four lines. `iptAcceptOffTailnet` counts a negated `-i`/`-s`/`-d` as off the tailnet, via the new `iptNegated`.
  - Two `TestReadIptablesUnscoped` rows (negated interface, negated source): red before the fix.

PR-Agent on #704 (head e6fbbcb; its run is marked partial):
- **Misleading message (REAL).** Applied. A read that answers with no count now says "returned no rule count (...)", and its fix lists the enabled rules instead of sending the operator to a failing read. `TestWindowsBuiltinGroup` asserts it on the two `unparsed` rows, which were red before the fix.
- **Misplaced doc comment (REAL).** Applied: a blank line now separates `TestWindowsBuiltinGroup`'s comment from `groupClean`'s.
- **Vacuous e2e guard (THEORETICAL).** Declined. In the same run, `windows.ps1` asserts `(RdGroupOn) -gt 0` once the group is enabled (the drift case near line 124). A query that resolves nothing on this image therefore fails that run, so the two `-eq 0` cases cannot pass alone.

## Review disposition (round 3, PASS-WITH-GAPS)

Reviewer `nan/deepseek-v4-flash`, at 94cf5d7 (`review.md`; earlier rounds kept as `review-round1.md` and `review-round2.md`).

- **Major THEORETICAL: the built-in group is read by its localized display name.** Ticketed as #707. On a non-English Windows, `-DisplayGroup 'Remote Desktop'` matches nothing and reads as `exposed=0`. The fix is the invariant `-Group` id, or `unverified` on an empty answer. It is not reproduced, because no localized guest exists here.
- **Minor REAL: the Windows e2e was not rerun at f08036a.** Applied: `make e2e-windows` at 94cf5d7 gave 52 pass, 0 fail (`scripts/e2e/out/windows-20261111T025924Z.log`).

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
- PR 3: iptables copies on a ufw host are deleted only when ufw is active and a scoped rule is in
  place. On inactive ufw only ufw's own list is replaced. A commented ufw rule is foreign, never
  exact, so a rule someone annotated is reported rather than deleted.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: the scope rule is ADR-017 Amendment B, and the off-tailnet gap is a missed case of it, not a new lesson
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017 Amendment B already records the scope; AC7 applies it
- [x] New pattern candidate for `00_meta/patterns/`? no: one project
