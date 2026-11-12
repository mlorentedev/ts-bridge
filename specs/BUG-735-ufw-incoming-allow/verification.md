---
tags: [spec, verification, templates]
created: "2026-11-12"
id: "BUG-735-ufw-incoming-allow"
type: spec-verification
status: active
owner: manu
---

# Verification - BUG-735-ufw-incoming-allow

## Evidence

- [x] AC1 -> commit `c8b5a57` / `TestUFWPolicy/incoming_allow:_both_policies_ACCEPT` (fix names `ufw allow <your SSH port>/tcp` and `ufw default deny incoming`, never `-P INPUT DROP`; `linuxFirewallOK` false; `Success` false)
- [x] AC2 -> commit `c8b5a57` / `TestUFWPolicy/IPV6=no_after_a_boot:_v6_ACCEPT_only` (fix names `IPV6=yes` and `ufw reload`)
- [x] AC3 -> `TestUFWPolicy/incoming_deny:_both_DROP`, the inactive and unreadable loop in `TestUFWPolicy`, `TestIP6Policy` (inactive ufw unchanged), `TestManualRewriteClearsSuccess`; review round 1 F3: commit `f66786a` / `TestUFWPolicyUnread`
- [x] AC4 -> `scripts/e2e/linux.sh` "ufw incoming allow" block (commit `35b258d`) / `make e2e-linux`: 103 pass, 0 fail on `4664ba6` (ufw incoming allow: manual naming `ufw default deny incoming`, not `firewall_ok`, setup leaves the default; ufw incoming deny: ok again)
- [x] AC5 -> commit `f66786a` / `TestUFWPolicyMessage`

## Test status

- `go test -race ./...` -> exit 0, every package ok
- `golangci-lint run` and `GOOS=linux golangci-lint run` -> 0 issues
- `make e2e-linux` -> 103 pass, 0 fail on `35b258d` (round 1); 103 pass, 0 fail on `4664ba6` (ufw incoming allow: manual naming `ufw default deny incoming`, not `firewall_ok`, setup leaves the default; ufw incoming deny: ok again)
- No regressions in existing test suite: yes

## Decisions made during implementation

- The kernel policy, not `ufw status verbose`, is the signal (deviation from the issue). Probed on the e2e guest, ufw 0.36.2: `deny (incoming)` -> `-P INPUT DROP` on v4 and v6; `allow (incoming)` -> ACCEPT on both; inactive -> ACCEPT; `IPV6=no` + reload leaves the v6 policy as it was. No new process, and the `IPV6=no` case is covered.
- On ufw the fix names ufw's commands, since ufw rewrites a raw `-P` on reload.
- Review round 1, F3 (applied, `f66786a`): an active ufw whose `iptables -S INPUT` fails while its own allow decides is `unverified` with the read's error and `firewall_ok` false. Before, it read `ok`.
- Review round 1, F4 (applied, `f66786a`): on ufw the note names ufw's incoming default (and `IPV6=no` for v6) instead of "unless another firewall filters this host"; the runbook row is keyed on that wording.
- Review round 1, F5 (declined): the fix keeps `ufw allow <your SSH port>/tcp` unscoped. A host whose operator reaches SSH over the LAN would be locked out by a tailscale0-only allow followed by `default deny incoming`; the advice matches #691's iptables fix. An operator who wants SSH on the tailnet only can scope it.

- Review round 2 (PASS), minor 1 (applied, `83c6623` and the commit after it): only a failed `iptables -S INPUT` marks the policy unread. A failed `ts-input` walk under a readable ACCEPT policy stays `manual` with ufw's fix (`TestUFWPolicyReadButChainUnread`), and a raw iptables host's unread step never names ufw.
- Review round 2, minor 2 (recorded): the `tasks.md` line says `TestManualRewriteClearsSuccess` gains the ufw rewrites; the ufw `Success=false` property is asserted in `TestUFWPolicy` (`s.Success == tt.manual`) and `TestUFWPolicyUnread` instead. `tasks.md` is left as reviewed.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: the policy-mirrors-default fact is recorded in ADR-017 Amendment B and the runbook row, and the bug shape is #691's
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017 Amendment B carries the line
- [x] New pattern candidate for `00_meta/patterns/`? no: single project

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-735-ufw-incoming-allow/` -> `specs/archive/BUG-735-ufw-incoming-allow/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
