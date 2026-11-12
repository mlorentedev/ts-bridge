# Autopilot report: ts-bridge-autopilot-2026-11-12

Objective: make the Linux firewall read tell the truth (#736, #733, #735, #739, #720, then #740).

## Ledger

- 07:10 armed. Before arming this session: #734 (#691) and #738 (archive BUG-526, HOST-011, BUG-691) merged; #713 measured on a Windows guest (no window, 6/6) and closed; #740 filed; leftover worktrees removed; #722 asked to recreate on master; both VMs confirmed free by feat-vm-linux-exec-3c.
- 11:45 #736 parked (Blocked): PR #742 draft at 575c03d, 4 spec-review rounds FAIL (each closed REAL defects; round 4: positive --sport range unreported). Verified: go test -race, lint, e2e-linux 108/0 at 8b93885. Options A land+ticket / B decide v4 model first (#743). Filed #743 (widened), #745. Note: session idle ~08:00-11:20Z, ~3h20m lost.
- 12:27 #733 done: PR #746 merged at b91b00d (v6 INPUT policy read on iptables hosts). Spec review PASS-WITH-GAPS (nan/deepseek-v4-flash); finding 3 applied (manual step gains v4 fix), CodeRabbit REAL applied (Success cleared on the 3 manual rewrites, incl. #691/#526 sites). PR-Agent needed `/review` (push gate skipped b91b00d); its REAL ufw IPV6=no finding ticketed on #735; nft twin noted on #739. go test -race, lint host+linux, e2e-linux 98/0 on 8814617 and b91b00d. Archive PR #747 open. Wake lock killed by an unknown SIGTERM ~12:15, re-armed to 19:15Z.
- 13:40 #735 done: PR #748 merged at 35bda99 (an active ufw's ACCEPT INPUT policy, v4 and v6/IPV6=no, reported as manual; unread policy reported as unverified). Spec review FAIL then PASS (nan/deepseek-v4-flash, both review files committed). CodeRabbit Major applied (IPv6 SSH lockout in the IPV6=no fix, probed on the guest: `ufw allow` adds the v6 copy cleanly only while ufw is disabled); PR-Agent THEORETICAL applied (ufw v6 note matched the raw ip6tables runbook row). Three complete PR-Agent runs (634491b, cb00719, 35bda99; the last two forced with `/review` past the push gate). Verified: go test -race, lint host+linux, e2e-linux 103/0 on cb00719 and 35bda99. #735 Done. Archive PR #749 opened. Next: #739 claimed.
- 14:10 #739 done: PR #750 merged at ed47300 (firewalld or a native nftables input table: check unverified and setup manual naming it, no write, neither policy nor v6 read; a failed probe is unknown). Spec review FAIL (AC5 unrun at review time, minors) then PASS (nan/deepseek-v4-flash, both files committed). firewalld probed by hand on a guest with the built binary (state/exit 252, zone commands: --change-interface, not --add-interface). PR-Agent full run on ed47300: 2 THEORETICAL declined with evidence; CodeRabbit rate-limited, no review. Verified: go test -race, lint host+linux, e2e-linux 107/0 on 999c0a7 and 67fe2b2. #749 (BUG-735 archive) landed unreviewed: PR-Agent ignores specs/, CodeRabbit rate-limited. Archive PR #751 opened.
- 17:05 #720 done: PR #752 merged at 5eecf2a (the IPv6 exposure scan follows INPUT's jumps into other chains, recursively, each chain once; a jump is followed only when it applies to all the port's traffic; ufw6-* chains are skipped on ufw hosts only). tailscaled's real v6 ts-input captured with capture-tsfw.sh (tailscale 1.104.4) and used as fixtures. Spec review PASS (nan/deepseek-v4-flash); minors applied, port-less accept question filed as #753. CodeRabbit 2 REAL applied (ufw6 skip without a ufw read; conntrack-qualified port jump followed), 1 declined (asked to edit review.md). PR-Agent full run on 73bb484: no issues. Verified: go test -race, lint host+linux, e2e-linux 112/0 on e9e8c7b and 85e640a. #747 #749 #751 archives merged. #720 Done. Archive PR #754 open. Manu returned ~16:20Z; awaiting him: #736 A/B, #753 A/B, whether to keep the run going.
- 17:22 #754 (BUG-720 archive) landed at b326521, unreviewed by the PR bots (CodeRabbit skipped, PR-Agent ignores specs/); disclosed in its triage. #740 not started: Manu is back, waiting for his go.
- 19:21 expired (19:10Z). Run closed: report, handoff, e2e-clean, cron deleted.

## Outcome

Queue: 736 parked, 733/735/739/720 done, 740 not started.

- Merged: #746 (#733), #748 (#735), #750 (#739), #752 (#720), and the archive PRs #747, #749, #751, #754.
- Parked: #736 on PR #742 (draft), after 4 spec-review FAIL rounds. Options: (A) land and ticket the `--sport` gap, or (B) decide the v4 model first (#743).
- Filed this run: #743, #745, #753.
- Landed unreviewed by the PR bots, each disclosed in its triage: #749, #751, #754 (spec archives: PR-Agent ignores specs/, CodeRabbit skipped or rate-limited); #750 (CodeRabbit rate-limited; PR-Agent ran).
- #722 unchanged since 07:08Z (DIRTY, needs recreating on master).
- #740 (Windows) not started. Manu returned at about 16:20Z and gave no go to start it.
- Lost time: the session sat idle from about 08:00 to 11:20Z. An unknown SIGTERM killed the wake-lock at about 12:15; it was re-armed.

## Awaiting Manu

1. #736: option A or B on #742.
2. #753: count a port-less v6 ACCEPT as exposure (A), or keep it as the operator's posture and document the limit (B).
3. #740: start it in the next session (measure on a Windows guest first).
4. #722: recreate it on master.

