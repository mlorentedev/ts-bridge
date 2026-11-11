# ts-bridge autopilot 2026-11-11

Armed 2026-11-11T07:53Z by Manu ("12h autopilot, continue the current line, file tickets for it"). Expires 19:53Z.

## Ledger

- 07:53 armed. Before arming this session: #710 merged (REL-376), #711 merged by Manu, #716 opened (#707, Windows e2e 53/0), #717 filed (Headscale discover endpoint 404s), #565 measured on the local mesh.
- 08:30 #716 (#707): PR-Agent finding applied (resource-id guard test), triaged, master merged; waiting for CI.
- 08:30 #714 (BUG-526): four findings applied or declined, a partial PR-Agent run added two: f1 replay fixed (REAL), v6 jumped chains ticketed #720. features.json changed, so BUG-526 spec review round 2 is running. PR-Agent marks its runs on this PR `partial`: it will not land until a complete run covers the head.
- 08:30 #715: PR-Agent skips it by design (no analyzable files). Left open for Manu, triage posted saying it is unreviewed.
- 08:30 #565 -> #719 opened: Self.Online poller, mesh e2e outage case 5/0 (503 after 20 s, 200 after 19 s), lesson 058. HOST-011 spec review running.
