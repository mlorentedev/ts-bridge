---
tags: [spec, verification, templates]
created: "2026-10-30"
---

# Verification - BUG-492-ts-input-chain

## Evidence

- [x] AC1 -> `TestIptablesTSInput/tailscaled default`; `TestIptablesRule/present inside ts-input`; `TestSetupForLinux/iptables, tailscaled's ts-input accepts the tailnet` (`ok`, no `run`); `TestCheckForLinux/iptables, tailscaled's ts-input accepts the tailnet` (`ok`, `firewall_ok` true).
- [x] AC2 -> `TestIptablesTSInput` (`deny before the jump decides first`, `deny after the jump is shadowed`, `tailscaled's other rules decide nothing`, `and an INPUT allow after the jump still counts`).
- [x] AC3 -> `TestIptablesTSInput` (`a qualified jump is not followed`, `no jump, nothing to follow`); `TestSetupForLinux` (`iptables, no ts-input jump: one read`, `iptables, a qualified ts-input jump is not followed`: the call lists hold no `iptables -S ts-input`). Every existing iptables row whose fixture jumps now expects the second read.
- [x] AC4 -> `TestIptablesRule/ts-input unreadable` (unknown); `TestSetupForLinux/iptables, ts-input unreadable: nothing appended` (`failed`, no `run`); `TestCheckForLinux/iptables, ts-input unreadable` (`unverified`). The reason reads `iptables -S ts-input: <err>`.
- [x] AC5 -> `tsInputDefault`/`tsInputNoAccept` in `linux_steps_test.go`, the printed form recorded on #492.

## Test status

- Red first: with `iptInlineChain` stubbed to return INPUT unchanged, 19 cases fail (the new ts-input cases and every existing iptables row that now expects the second read). The rows that pin unchanged behaviour (deny before the jump, qualified jump, no jump) pass against the stub, as intended.
- Mutation checks, one at a time and reverted. Each one fails at least one case:
  - following any jump that ends in `ts-input` fails the qualified-jump rows;
  - ignoring the chain's read error fails the three `unreadable` rows;
  - appending the chain at the end instead of at the jump fails `deny after the jump is shadowed`;
  - reading the chain but not using it fails the three `accepts the tailnet` rows.
- `go test -race -count=1 ./...` -> all packages ok.
- `GOOS=linux|windows|darwin go vet ./...` -> clean. `golangci-lint run` -> 0 issues. `gofmt -l internal/host` -> empty. The eight files `gofmt -l .` lists elsewhere predate this change (#466).
- End to end in `ubuntu:24.04` (`NET_ADMIN`), with a `GOOS=linux` binary and tailscaled's rules replayed:
  - `host setup`: "Firewall allows TCP 3389 (iptables)", and `iptables -S INPUT` is unchanged (`-P INPUT ACCEPT`, `-A INPUT -j ts-input`);
  - `host check --json`: the firewall step is `ok` and `firewall_ok` is true. The exit status is 2 because the container has no xrdp and no Tailscale;
  - with the tailnet ACCEPT deleted from `ts-input`, `host setup` appends `-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT` as before (`changed`).

## Decisions made during implementation

- `iptablesDecide` is unchanged. The chain is spliced into the INPUT text at the jump (`iptInlineChain`), so BUG-035/483/481's rules apply to its lines without a second walker. `-N ts-input` is skipped like any non-`-A` line.
- The lesson is numbered 050 because the HOST-005 A2 branch (#458, a parallel session) has lesson-049 drafted. `check-lessons.sh` allows gaps but rejects duplicates.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-050-2026-10-30.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017 already says setup appends only when no allow covers the port; reading tailscaled's chain widens what "covers" reads, not the contract.
- [x] New pattern candidate for `00_meta/patterns/`? no: "follow only the indirection whose contents you know" is ts-bridge firewall detail here, and lesson 046 already carries the general allow-list point.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-492-ts-input-chain/` -> `specs/archive/BUG-492-ts-input-chain/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
