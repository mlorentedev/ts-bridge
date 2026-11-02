---
tags: [spec, verification]
created: "2026-11-01"
---

# Verification - BUG-480-iptables-duplicate-rules

## Evidence

- [x] AC1 -> `TestCheckForLinux/iptables, two copies of the setup rule`. The step is `manual`, the message contains `2 copies`, the fix contains ``iptables -D INPUT -p tcp --dport 3389 -j ACCEPT` 1 time``, and `firewall_ok` is true in both the services JSON and the legacy keys.
- [x] AC2 -> `TestIptablesSetupCopies` (`one`, `a scoped allow is not a copy`, `another port is not a copy`, `a drop on the port is not a copy`, `setup's own command line is not the printed form`) and `TestCheckForLinux/iptables, one copy beside a scoped allow` (`ok`).
- [x] AC3 -> `TestCheckForLinux/iptables, two copies behind tailscaled's ts-input` (`manual`, 2 copies, with `tsInputDefault`) and `TestIptablesSetupCopies/ts-input rules are not copies`.
- [x] AC4 -> `TestCheckForLinux/iptables, a drop before two copies`. The step is the blocked `manual`, it names the DROP, and the message does not contain `copies`.
- [x] AC5 -> `TestSetupForLinux/iptables, two copies of the setup rule`. Setup reports `ok`, and the call list holds no `run iptables -D`.
- [x] AC6 -> `iptSetupRule` is the line iptables 1.8.10 printed after `iptables -A INPUT -p tcp --dport 3389 -j ACCEPT` in the end-to-end run below. The ts-input fixtures are BUG-492's.

## Test status

- Red first: with `iptSetupCopies` stubbed to return 0, 9 subtests fail (11 `--- FAIL` lines with their parents). They are the two check rows with copies and seven `TestIptablesSetupCopies` rows. The rows that pin unchanged behaviour (one copy beside a scoped allow, a drop before the copies, setup `ok`, and the stub's `none`/command-line rows) pass against the stub, as intended.
- Mutation checks, one at a time, each reverted with the suite back to `ok`. Every mutant built and failed at least one named case, and every result below was re-run through `checkFor`/`setupFor`, not only the helper:
  - with the threshold `copies > 2`, both two-copy check rows fail;
  - with the printed form equal to setup's command line (no `-m tcp` inserted), both two-copy check rows and eight `TestIptablesSetupCopies` rows fail;
  - when the fix counts `copies` instead of `copies-1`, both two-copy check rows fail on the fix;
  - when `copies` is never carried in `firewallRead`, both two-copy check rows fail;
  - when setup also reports copies (`firewallCopiesStep` in the `rulePresent` case of `firewallSetupStep`), the setup row fails;
  - with a loose match (any line holding `--dport 3389` and ending `-j ACCEPT`), `TestCheckForLinux/iptables, one copy beside a scoped allow` fails, along with `a scoped allow is not a copy`, `another port is not a copy` and `setup's own command line is not the printed form`.
- `go test -race -count=1 ./...` passes on all packages. `GOOS=linux|windows|darwin go vet ./...` is clean. `golangci-lint run` reports 0 issues. `gofmt -l internal/host` is empty. `scripts/check-lessons.sh` reports `OK (50 lessons)`.
- End to end in `ubuntu:24.04` (`NET_ADMIN`) with a `GOOS=linux` binary, after `iptables -A INPUT -p tcp --dport 3389 -j ACCEPT` was run twice:
  - `iptables -S INPUT` prints `-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT` twice;
  - `host check --json` reports the firewall step as `manual`, with "Firewall allows TCP 3389 (iptables), but holds 2 copies of -A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT" and the `-D … 1 time(s)` fix, and exits 2. The container has no xrdp and no Tailscale, so those steps fail as well;
  - `host setup` reports "Firewall allows TCP 3389 (iptables)", and both copies are still there afterwards;
  - after one `iptables -D INPUT -p tcp --dport 3389 -j ACCEPT`, `host check --json` reports the firewall step as `ok` with `firewall_ok` true.

## Decisions made during implementation

- `manual`, not the `changed` that #480 proposes: on the check path `changed` means setup would converge, and setup deletes nothing. Recorded in the proposal's risks, together with the exit status: hosts set up more than once before #477 move from exit 0 to exit 2, as they would have under `changed`.
- `iptSetupPrinted` is derived from `iptablesAppend` (it inserts `-m tcp` after the protocol), so the counted form cannot drift from the rule setup writes. This also cleared two `goconst` findings that a hand-written literal had raised.
- `firewallCheckStep` tests `copies > 1` without also testing `rulePresent`. Unknown and blocked return before it, and a copy is an unscoped ACCEPT, so copies cannot exist on an absent read. A guard on `rulePresent` would be code no test can reach.
- `assertNamesBlockedRule` now accepts a `manual` step without a blocked rule, provided its message does not report a block. The copies step is the first `manual` firewall step that is not BUG-481's.

## Review round 1 (nan/deepseek-v4-flash, PASS at `38f3cd7`)

- Minor, the text report shows `Firewall: ✓ OK` above the `manual` copies step: declined as an accepted trade-off. The summary row prints `firewall_ok`, which means "the port is allowed", and that stays true. The `manual` step is printed in the same block, directly below, with the fix. Making the row depend on the step outcome would change the meaning of `firewall_ok` in text mode only, and the JSON contract (C2) keeps that meaning.
- Minor, iptables copies that predate a later ufw install are invisible because the ufw read is preferred: ticketed on #530 (comment of 2026-11-01). It has the same root cause as #530: the ufw read never looks at raw iptables, and that ticket's option 2 covers both cases.
- Minor, the `features.json` entries are still `pending`: declined. Only the harness may write `passing`, and the agent may not (template rule). This repo has no command that records the evidence, and the archived specs carry 102 `pending` entries against 7 `passing`. All four verification commands exit 0, as the reviewer confirmed.
- Minor, `iptSetupPrinted` used `slices.Index(rule, "tcp")`, so a missing protocol would quietly count zero: applied after the review (`34fe9e0`). The `-m` match is now inserted after the value of `-p` and takes that protocol's name. An off-by-one mutant of the new index fails 9 subtests. The change is code only, so the contract digests are unchanged.
- Minor, the AC boxes in `proposal.md` are unticked: declined. This repo's archived specs keep their ACs as `- [ ]` (BUG-035, BUG-481, BUG-492), and the evidence lives here.

## PR review (CodeRabbit on #534, at `1701981`)

Three nitpicks, all applied in `71f450d`, tests only. Each new row fails a mutant that the suite missed before:

- The copies message is asserted against the full printed rule, not just `2 copies`.
- `iptables, three copies: two to delete` pins the deletion count at `2 time`. It fails a fix whose count is hard-coded at 1.
- `the same rule inside ts-input is not a copy` makes chain scoping explicit. It fails a match that ignores the chain name.

PR-Agent published no review: #535 (a skipped run cancelled the reviewing one), then #430 (NaN timeout).

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-051-2026-11-01.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017 already defines `manual` as the outcome for state the operator must resolve; this applies it, it does not widen it.
- [x] New pattern candidate for `00_meta/patterns/`? no: "a check outcome promises what setup will do" is the ADR-017 outcome model in this repo, and lesson 051 carries it.

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/BUG-480-iptables-duplicate-rules/` -> `specs/archive/BUG-480-iptables-duplicate-rules/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [x] Promotions above executed (if any)
