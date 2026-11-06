# Overnight run ledger: ts-bridge-autopilot-2026-11-06

One line per item: item, outcome, PRs, notes.

- 08:40Z land-open-prs: merged #666 (6d805b2), #668 (debd0a1), #669 (3090051); none closes an issue (all "Part of" #630/#572). Re-review findings applied on #678 (7bf3a4c), #681 (f249e4a), #665 (8b20b8e), #682 (be71b50); #662 merge conflict with #666 resolved (686c976). CodeRabbit rate-limited throughout (notice, not review). #679/#667/#659/#664/#672/#670 in the second landing run.
- 08:40Z 631: stacked #686 -> #687 -> #688 -> #689. VM e2e: Linux 56/0 (#687), 66/0 (#688), Windows 49/0 (#689). #686 review has 3 findings (one REAL: upgraded hosts keep the built-in Remote Desktop group enabled), handed to the author lane for triage.
- 09:15Z land-open-prs: merged #667 (99406dd, Part of #630). #679 re-review finding applied (cdcaaa5: posix_acl_valid shape check). Landing waiter rewritten to wait on a pid: the pgrep-by-name waiter matched its own command line and would never have started.
- 09:15Z 631: #689 Windows e2e on 2fce851, 50/0 (create-before-remove, built-in group report). Closes #685 added to #689. #686 re-review raised 3 more findings (iptables ACCEPT policy, Windows OpenSSH built-in rule, removal identity), handed to the author lane.
- 09:15Z 391: draft #690 (Part of #391), per-user store lock plus control_url re-check inside it. CI and review pending; the Windows lock path still needs a VM run.
- 09:55Z land-open-prs: merged #670 (e38ae47, Closes #588 -> Done). Re-review findings applied on #659 (a288b6e), #672 (86ef8fa); #664 declined to #690 (key-kept assertion belongs to the store lock). #678 lint (goconst after master merge) fixed in 3bf6c0c. Each update-branch triggers a re-review that often raises a new finding, so landing converges one PR per pass.
