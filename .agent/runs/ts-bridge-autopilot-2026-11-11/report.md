# ts-bridge autopilot 2026-11-11

Armed 2026-11-11T07:53Z by Manu ("12h autopilot, continue the current line, file tickets for it"). Expires 19:53Z.

## Ledger

- 07:53 armed. Before arming this session: #710 merged (REL-376), #711 merged by Manu, #716 opened (#707, Windows e2e 53/0), #717 filed (Headscale discover endpoint 404s), #565 measured on the local mesh.
- 08:30 #716 (#707): PR-Agent finding applied (resource-id guard test), triaged, master merged; waiting for CI.
- 08:30 #714 (BUG-526): four findings applied or declined, a partial PR-Agent run added two: f1 replay fixed (REAL), v6 jumped chains ticketed #720. features.json changed, so BUG-526 spec review round 2 is running. PR-Agent marks its runs on this PR `partial`: it will not land until a complete run covers the head.
- 08:30 #715: PR-Agent skips it by design (no analyzable files). Left open for Manu, triage posted saying it is unreviewed.
- 08:30 #565 -> #719 opened: Self.Online poller, mesh e2e outage case 5/0 (503 after 20 s, 200 after 19 s), lesson 058. HOST-011 spec review running.
- 2026-11-12 01:35 closed after expiry (19:53Z passed with no ticks after 08:30; the loop stalled on a context compaction). State at close: #714 PR-Agent `review` red, runs still partial, BUG-526 round 2 PASS; #716 `smoke` red on an apt mirror DNS failure (runner, not code) and `hygiene` red; #719 `hygiene` red, HOST-011 review PASS with one Minor (record PR in verification.md); #715 merged by Manu; #717 committed locally (dc5c2d9, mesh e2e 4/0) and not pushed. Not done: host-008-handoff, #713, mesh rdp flake ticket. `make e2e-clean` run.
- 2026-11-12 02:50 (interactive, Manu back): #716 merged (#707 Done); #731 merged (#717 Done, mesh e2e 4/0); #719 all reviewer output triaged in three comments, HOST-011 spec review round 2 PASS, mesh e2e on the master merge 6/0, but PR-Agent still marks every run partial: left for Manu with #714. Tickets filed: #725 (mesh rdp flake), #726 (silent control outage), #730 (outage-case shim, shared health address). #522 commented with measured facts.
- 2026-11-12 04:30 (interactive): Manu merged #714 and #719 (#526, #565 Done).
  - #719: the CodeRabbit outage-budget finding was applied (fb3f18d, mesh e2e 6/0). The bus re-attach finding was ticketed #732.
  - #691 -> #734 merged (Done): a permissive iptables INPUT policy is reported `manual`, and setup never runs `-P`.
    - Linux e2e 94/0, three times.
    - The spec review round 1 was PASS-WITH-GAPS. All six findings were dispositioned, and F1 (a comment forging the policy line) was fixed.
    - Three PR-Agent rounds were applied: the fix-text guard, the loopback/ESTABLISHED/SSH-port remediation, and the hedge for another firewall.
  - #738 merged: archives BUG-526 (fresh round-2 review PASS-WITH-GAPS; F2/F3/F6 applied), HOST-011 and BUG-691.
  - Tickets filed: #733 (v6 policy), #735 (ufw default allow), #736 (negated or portless allows invisible to the exposure reads), #737 (capture driver shim), #739 (firewalld/nftables detection).
  - Worktrees for #691 and the archive were removed.
  - Kept for Manu: `fix-issue-565-control-liveness` (untracked round-1 review transcripts) and `fix-issue-526-tailscaled-firewall-modes`. The second holds an uncommitted BUG-526 `review.md` (PASS at 42b131f) from the autopilot's round 2; the archive carries the later round, PASS-WITH-GAPS at the merged head.
  - Still open from the queue: host-008-handoff, #713.
