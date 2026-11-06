# Overnight run ledger: ts-bridge-autopilot-2026-11-06

One line per item: item, outcome, PRs, notes.

- 08:40Z land-open-prs: merged #666 (6d805b2), #668 (debd0a1), #669 (3090051); none closes an issue (all "Part of" #630/#572). Re-review findings applied on #678 (7bf3a4c), #681 (f249e4a), #665 (8b20b8e), #682 (be71b50); #662 merge conflict with #666 resolved (686c976). CodeRabbit rate-limited throughout (notice, not review). #679/#667/#659/#664/#672/#670 in the second landing run.
- 08:40Z 631: stacked #686 -> #687 -> #688 -> #689. VM e2e: Linux 56/0 (#687), 66/0 (#688), Windows 49/0 (#689). #686 review has 3 findings (one REAL: upgraded hosts keep the built-in Remote Desktop group enabled), handed to the author lane for triage.
- 09:15Z land-open-prs: merged #667 (99406dd, Part of #630). #679 re-review finding applied (cdcaaa5: posix_acl_valid shape check). Landing waiter rewritten to wait on a pid: the pgrep-by-name waiter matched its own command line and would never have started.
- 09:15Z 631: #689 Windows e2e on 2fce851, 50/0 (create-before-remove, built-in group report). Closes #685 added to #689. #686 re-review raised 3 more findings (iptables ACCEPT policy, Windows OpenSSH built-in rule, removal identity), handed to the author lane.
- 09:15Z 391: draft #690 (Part of #391), per-user store lock plus control_url re-check inside it. CI and review pending; the Windows lock path still needs a VM run.
- 09:55Z land-open-prs: merged #670 (e38ae47, Closes #588 -> Done). Re-review findings applied on #659 (a288b6e), #672 (86ef8fa); #664 declined to #690 (key-kept assertion belongs to the store lock). #678 lint (goconst after master merge) fixed in 3bf6c0c. Each update-branch triggers a re-review that often raises a new finding, so landing converges one PR per pass.
- 10:15Z #681 re-review finding applied (1847b5d). #690 CI green including test-windows, so the store lock now runs on Windows in CI; it is waiting to be marked ready and reviewed. Queue check: #684 waits on #665 (both edit discover.go), #526 waits on the #631 stack (both edit the Linux firewall reader), and #565 is vetoed for this run because it needs a live control-plane outage (kubelab#2862 or the #607 substrate).
- 10:40Z Policy change after an advisor review. Re-review findings are now dispositioned without a push unless they are REAL: a THEORETICAL finding is declined or ticketed (one ticket per PR), because every push starts another review round. Filed #692 (from #662) and #693 (from #665). The #682 spec checkbox was corrected (cb031a2). #679's evidence line was amended: the shape check is reasoned from the kernel and was not VM-observed on cdcaaa5. #690 Windows e2e 45/0 on 33b60c8. #689's head moved to 641f2d0 after its 50/0 run, so it needs a new Windows run before it lands.
- Budget note: landing converges at about one PR per 25 min, because each update-branch starts a PR-Agent re-review. Not all 14 open PRs will land in this window. Iterations are counted as queue items worked (land-open-prs, 631, 391 so far), not loop ticks.
- 11:45Z land-open-prs: merged #679 (635945a, Closes #598), #659 (3fd908d, Closes #633), #664 (398ead6, Closes #653), #678 (fd06fd5, Closes #601) and #665 (7165991, Closes #646). All five issues moved to Done. The THEORETICAL re-review findings on #672 and #682 were ticketed as #694 and #695, without a push; both are queued to land. The teammates for 631 and 391 hit their usage limit, so the lead took over their work.
- 11:45Z 631: #686 triaged (9 findings). 3 REAL ones were applied in fd4392c (the features.json check is anchored, PR 4 must close #685, Linux ssh is marked planned in §5). The other 6 were already fixed at this head.
- 11:45Z 391: #690 merged master with #664 folded in (758fba0). The share check now runs inside the BindCredential transaction, with a red-first test; AssociateCredential is removed. Test, lint and gosec are green.
- 12:20Z land-open-prs: merged #662 (da7f8af, Closes #628), #672 (f77459d, Closes #638) and #682 (9ec0c90, Part of #607). #628 and #638 moved to Done. On #681 and #686, re-review findings were declined with reasons and no push; both are in land run 8.
- 12:20Z 391: #690 REAL finding applied in 9357777: the bind refuses to replace a stored key that exists but cannot be read, with a red-first test. The "vacuous assertion" finding was declined because the assertion can fail. The triage is posted; the re-review is pending.
- 12:20Z 607: opened PR 2 as #696 (Part of #607), merged with master. `make e2e-mesh-linux` passed at 753e19a: 3 pass, 0 fail, no leftover container. The first attempt failed for an environmental reason: the worktree was under /private/tmp, which the Docker VM does not share. The worktree was moved under ~/Projects. PR 3 commit cf83d90 stays local on test/issue-607-mesh-linux (not run live).
- 13:22Z land-open-prs: merged #686 (80108b2, part 1 of the #631 stack), #681 (02367c8, Closes #680 -> Done) and #696 (369321c, #607 PR 2). #687 was merged with master after the #686 squash; six conflicts resolved, two of them as unions. `make e2e-linux` passed at 6e960d7, 67 pass and 0 fail, posted on the PR. It was then marked ready, because PR-Agent skips drafts. The #696 REAL findings were applied in a689131. On #690 and #687, PR-Agent repeatedly "succeeded" without publishing a review; its log suspects NaN concurrency exhaustion. Both jobs were re-run, and land run 10 is queued for them.
- 13:56Z land-open-prs: merged #687 (605e5d2, Part of #631) after REAL findings were applied in 69690fe. The first was an ACCEPT on another interface or source counted as the tailnet rule, so setup could delete the legacy copy without adding `-i tailscale0`. The second was a PR 3 checkbox copied in by the merge. `make e2e-linux` passed at 69690fe, 67 pass and 0 fail. **The PR-Agent review on 69690fe failed without publishing, and land.sh still merged it.** land.sh accepted any completed review, and `dotf pr triage-queue` reported the queue clear. This is disclosed on #687. land.sh now requires `review=success`; run 11 was stopped, and #690 goes through the stricter run 12.

## Disarm report (written 14:00Z)

Stop reason: the landing window closed at 14:00Z, as planned before expires_at 14:55Z. The envelope was unchanged on every tick. The wake lock (caffeinate) covered the whole window.

### Merged during the run (06:55Z to 14:00Z)

- #667 99406dd (Part of #630), #668 debd0a1, #669 3090051, #670 e38ae47 (Closes #588).
- #679 635945a (Closes #598), #659 3fd908d (Closes #633), #664 398ead6 (Closes #653), #678 fd06fd5 (Closes #601), #665 7165991 (Closes #646).
- #662 da7f8af (Closes #628), #672 f77459d (Closes #638), #682 9ec0c90 (#607 PR 1), #686 80108b2 (#631 PR 1), #681 02367c8 (Closes #680).
- #696 369321c (#607 PR 2), #687 605e5d2 (#631 PR 2).

That is 16 PRs. Every closed issue is moved to Done on the board.

### Open, with the next step

- **#690** (#391 store lock): REAL findings were applied (the #664 fold in 758fba0; the unreadable-key refusal in 9357777). Its review job failed to publish three times. The next step is the result of land run 12. If that run does not merge it, re-run the `review` job, triage what it finds, and then land.
- **#688** (#631 PR 3, ufw) is a draft, now based on master. Next: merge origin/master (expect the same spec conflicts as #687), run `make e2e-linux`, mark it ready, triage, land.
- **#689** (#631 PR 4, Windows) is a draft based on #688. After #688 lands: retarget it, merge master, re-run the Windows e2e on the new head (message the VM peer first), triage, then land. It must keep `Closes #631` and `Closes #685`.
- **#607 PR 3**: commit cf83d90 is local on `test/issue-607-mesh-linux`. It needs its live Lima run and mutants before it gets a PR. PR 4 (Windows mesh) and PR 5 (runbook) follow the spec.
- **#676** (complexity baseline): it has conflicts and an untriaged review. It was not reached.
- Not mine: #523 (release), #583, #584, #674, #675, #677.

### Queue items not started

#684, #526 and #565. #684 was blocked on #665, which has now merged, so it is free for the next run. #526 is blocked on the #631 stack. #565 was vetoed for this run because it needs a live control-plane outage.

### Iterations

Seven queue items were worked: land-open-prs, 631, 391, 607, plus the #686, #687 and #690 sub-steps inside them. No item was parked.

### Lessons for the operator

- Serial landing converges at about one PR per 25 minutes. Every update-branch starts a new PR-Agent review, and that review often raises a new finding. The trigger configuration lives in `.github/`, which is yours to change.
- PR-Agent "succeeded but published no review" about six times today, and its log suspects NaN concurrency exhaustion. `dotf pr triage-queue` does not treat a failed review on the head as pending, so a script that trusts it can merge an unreviewed commit (#687). The fix belongs in dotf, the owner of triage-queue; it is filed as dotfiles#3243, and land.sh is now stricter locally.
- A worktree under /private/tmp is not shared into the Docker VM. Headscale then starts with an empty config and fails with "no config file found". Keep e2e worktrees under ~/Projects.
