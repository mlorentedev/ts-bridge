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

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-051-2026-11-01.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017 already defines `manual` as the outcome for state the operator must resolve; this applies it, it does not widen it.
- [x] New pattern candidate for `00_meta/patterns/`? no: "a check outcome promises what setup will do" is the ADR-017 outcome model in this repo, and lesson 051 carries it.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-480-iptables-duplicate-rules/` -> `specs/archive/BUG-480-iptables-duplicate-rules/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
