---
tags: [spec, verification, templates]
created: "2026-11-12"
id: "BUG-733-v6-input-policy"
type: spec-verification
status: active
owner: manu
---

# Verification - BUG-733-v6-input-policy

## Evidence

- [x] AC1 -> commits `318627f`, `8814617` / `TestIP6Policy` rows `v4 DROP, v6 ACCEPT, rule present` and `v4 DROP, v6 ACCEPT, rule absent: setup adds it and reports manual` (the recording runner sees no ` -P ` call)
- [x] AC2 -> `TestIP6Policy/an unfollowable v4 jump keeps its outcome`, the ufw assertion at the end of `TestIP6Policy`, `TestIP6Exposure/v6 read fails`, `TestIP6Exposure/no ip6tables`, `TestPermissiveInputPolicy`
- [x] AC3 -> `TestIP6Policy/both ACCEPT: one manual step naming both policies`; `TestPolicyFixOnManualStep` (commit `5c76db3`) extends it to a step already manual for another reason
- [x] AC4 -> `scripts/e2e/linux.sh` v6 permissive-policy block / `make e2e-linux`: 98 pass, 0 fail, 0 xfail, 0 xpass on candidate `8814617` (v6 permissive policy: manual naming the ip6tables policy, not `firewall_ok`; closed v6 policy: DROP after the keep-open rules)

## Test status

- `go test -race ./...` -> exit 0, every package ok
- `golangci-lint run` and `GOOS=linux golangci-lint run` -> 0 issues
- `make e2e-linux` -> 98 pass, 0 fail, 0 xfail, 0 xpass on candidate `8814617` (v6 permissive policy: manual naming the ip6tables policy, not `firewall_ok`; closed v6 policy: DROP after the keep-open rules)
- No regressions in existing test suite: yes

## Decisions made during implementation

- ICMPv6 joined the v6 keep-open rules (`a9b06a3`, `8814617`): a v6 DROP without it cuts neighbor discovery and the host off IPv6. `proposal.md` still lists loopback, replies and SSH only; it was left as reviewed so the verdict stays fresh, and the code, ADR-017, runbook and tests carry ICMPv6.
- `TestIP6Exposure` moved to an `ip6Closed` fixture (v6 policy DROP), so it keeps testing the rules scan alone; the policy behaviour lives in `TestIP6Policy`.
- Review finding 3 (applied, `5c76db3`): `withPolicy` now gives an already `manual` step the v4 policy fix too, as `withV6Policy` does, so a host with a foreign allow and both policies open is told to close both.
- Review finding 4 (ticketed): a false positive where firewalld, native nftables, or legacy binaries beside tailscaled's nftables mode leave `ip6tables -S INPUT` at ACCEPT over an empty chain. It is the v6 twin of #739 and recorded there.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: the bug is #691's shape on the second address family; ADR-017 Amendment B and the runbook row record the ICMPv6 caveat
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017 Amendment B already carries the line, updated in `025a869`
- [x] New pattern candidate for `00_meta/patterns/`? no: single project

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/BUG-733-v6-input-policy/` -> `specs/archive/BUG-733-v6-input-policy/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [x] Promotions above executed (if any)
