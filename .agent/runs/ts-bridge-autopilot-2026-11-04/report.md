# Run report: ts-bridge-autopilot-2026-11-04

Armed 2026-11-04T15:45Z, expires 23:45Z. One ledger line per item.

## Ledger

- 15:50Z HOST-005-archive: in progress. No review.md on master (round 1 was FAIL on 70251c9). Round 2 launched with `dotf spec review HOST-005-host-config --reviewer nan/deepseek-v4-flash --foreground` in worktree chore-archive-host-005 (tmux launch died: tmux server cwd deleted, so foreground).
- 16:05Z #415: PR #586 open, ready. go test -race ./... ok, golangci-lint 0 issues, gosec clean. Awaiting CI + reviewers.
- 16:20Z HOST-005-archive: PARKED (needs Manu). Round 2 FAIL (deepseek-v4-flash, 42c777b; review.md kept untracked in worktree chore-archive-host-005). Major AC6 (no descriptor in `setup --json`) fixed in PR #587. Major release marker: #579 merged as plain `feat(host):`, so release #523 (1.21.0) files the breaking `host init` change under Features. Options A (BEGIN_COMMIT_OVERRIDE on #579, bump to 2.0.0) or B (keep 1.21.0, prose note) are posted on #458. Minor function length filed as #588.
- 16:40Z #573: PR #590 open, ready. Flake reproduced (bound 127.0.0.1:1055 -> `Merge() error = <nil>`), fixed via the existing selectAvailablePort seam; passes with 1055 bound. Root cause side defect filed as #589 (Merge swallows the port-range derivation error).
- 16:45Z #415/#586: reviewer round 1 applied (partial-write test, close-log totals, ReaderFrom comment corrected), head 9ab4703. #587: PR-Agent REAL finding applied (hostTailscaleIP seam, command-level test, mutant killed), head 3e99424.
- ~17:00Z #586/#587/#590: triaged (## Review triage posted). All three blocked only by `test-macos` cancelled with no runner assigned (0 steps, 16 min queue): GitHub-hosted macos-latest capacity, not code. Failed jobs re-run at 16:56Z. (Earlier ledger times 16:05-16:45Z were estimates; the clock read 16:22Z at the first loop tick.)
- ~17:05Z #500: PR #591 open, ready. The suite re-executes itself under bash and zsh (TGT_SHELLS), so no .github change was needed; it fails on a missing shell or an inner failure (evidence in the PR body).
- 17:15Z #573: MERGED #590 at 1de2425 (dotf pr land), board Done, worktree removed. #586 BEHIND, branch updated (new CI round). #587/#591 test-macos cancelled again with no runner; failed jobs re-run.
- 17:28Z #591 triage posted, branch updated. #586 triage refreshed for head 870679a (Doc drift declined with git-grep evidence); queue clear for #586.
- 17:28Z #455: spec HOST-455-windows-host-state filled (3 PRs W1→W2→W3). W1 = PR #594 (ready): tailscale set --unattended after reading ForceDaemon; lesson-053; mutant killed. Filed #593 (site/ copy of the old command; site/** frozen).
- 17:34Z #455: W2 committed on fix/issue-455-rdp-deny-state (aeef3c6; policy-first fDenyTSConnections, 2 mutants killed). W3 committed on fix/issue-455-firewall-read (f166b50; read-first rule, action/direction checked, 2 mutants killed). Both pushed, held as branches: their PRs open once #594 lands, because stacked PRs get no CI (lesson-052). #587 updated from master.
- 17:44Z #561: claimed (board In Progress), spec HOST-561-state-dir-ancestors, PR #596 ready (lexical ancestor walk in CheckStateDir+EnsureStateDir, 2 mutants killed). Filed #595 (same gap for auth_key_file). #586 land attempted: refused UNSTABLE, test-macos pending (non-required check blocks land; tooling gap noted).
- 18:00Z #376: spec REL-376-contain-ssh-bootstrap, PR #597 ready (Linux Pdeathsig, Windows kill-on-close Job Object released after Wait, macOS boundary in the runbook; 137 executable LOC; Pdeathsig mutant killed in Docker Linux; Windows evidence = test-windows job). Board In Progress.
- 18:05Z MERGED #587 at e67d994 (dotf pr land) after a re-triage of PR-Agent's re-run. #591 re-triaged, BEHIND → branch updated. #594 hygiene red: lesson 053 missing from docs/lessons/_index.md → indexed (0a93167). #586 test-macos cancelled: re-run 1/3.
- 18:45Z triage round. Applied: #591 fails when TGT_SHELLS names no shell (610f4b9); #596 follows a root-owned ancestor link to its target (macOS /tmp -> /private/tmp 1777 now refused; 3-case mutant killed; 91ef788); #597 names a containment refusal and documents the pre-assignment window. #594: both features.json findings declined (harness-owned state; W2/W3 tests are planned). #594 updated from master. #586 test-macos queued 15 min with no runner and was cancelled again: re-run 2/3.
- 19:10Z #376: MERGED #597 at d77cc17 (dotf pr land), board Done, worktree removed by land.
- 19:15Z re-triage round. #594: the `prefs unreadable` case now fails a mutant that trusts a failed read's stdout; mutation claim corrected 3→2 (8dfa67e). #596: real-fs walk tests skip on a root-only TMPDIR (40b40fe). #591: PR body verification re-run, now exercising `failed under` (no code change). #591 land refused BEHIND (after #597) → branch updated; #586 updated too. Every land makes the rest BEHIND, which restarts the runner-starved test-macos.
- 19:45Z re-triage round 3. #586: CodeRabbit's accumulation nitpick applied as a unit test (7f07f7d, mutant killed). #596: spec and runbook aligned with the link-following walk (b272f93). #594: PR-Agent's mutant objection declined with both mutants re-run. #591: repeated guard-bypass declined; test-macos queued 17 min with no runner → re-run 2/3. #594 updated from master. PR-Agent re-reviews after every master merge, so each update reopens the triage queue.
- 19:55Z #500: MERGED #591 at 25a66fa (dotf pr land), board Done, worktree removed by land. #596: skip probe fixed, tasks.md aligned, ACL limit documented (3790694); filed #598 (POSIX ACLs ignored on Unix ancestors). #586: doc-drift finding declined with git-grep evidence; land refused BEHIND after #591 → branch updated (3rd CI round).
- 20:00Z #415: MERGED #586 at 51f2d8f (dotf pr land), board Done. #596 and #594 updated from master (#594's test-macos had been cancelled with no runner; the update supersedes the re-run).
- 20:15Z #594 and #596: test-macos cancelled after ~15 min queued with no runner (the same pattern on every PR today) → re-run 1/3 each. The cancel always lands about 15 min after queueing, which looks like a queue/timeout limit in ci.yml; that is .github/**, so it is reported, not touched.
- 20:45Z #455 W1: MERGED #594 at 2771d84 (repeated findings declined with mutant evidence). W2: master merged into fix/issue-455-rdp-deny-state (spec conflicts resolved keeping W1's corrected mutant claim and W2's AC2 evidence), PR #599 ready, "Part of #455". Its Knowledge line first claimed the runbook already covered Group Policy; it did not, so a runbook note was added (06058dd). W3 waits for #599. #596: test comment fixed (8cf6bf0), "absent ancestor" finding declined (createStateDir runs MkdirAll first), master merged.
- 21:00Z #455 W2: MERGED #599 (dotf pr land). W3: master merged into fix/issue-455-firewall-read (W3 code kept; master's corrected W1 mutant claim and `prefs unreadable` case kept), PR #600 ready with `Closes #455`.
- 21:40Z #561: MERGED #596 at 74391be (dotf pr land), board Done. CodeRabbit's second round was real and applied (75ca92d): a link target's `..` was cleaned lexically, so `/srv/link -> /srv/alias/../store` was checked at `/srv/store`; nested links cost 3071 lstat calls at depth 10. The walk now resolves top-down as path_resolution(7) with one 40-link budget, and a `..` in state_dir is refused. PR-Agent's link-suffix test gap applied (c2da615, mutant killed); its relative-path and absent-ancestor findings declined three times with mesh.go:79 / statedir_unix.go:168. #600: review job published nothing twice (NaN exhaustion per the job log) → re-run 2/3.
- 22:05Z #455 W3: MERGED #600 at 9719bad (dotf pr land); #455 closed, board Done. Three review rounds, all real findings applied: the read now compares protocol (TCP), profile (Any) and remote address (Any) as well, since the recreate sets them and the old always-recreate normalised them; check names a present-but-drifted rule; setup and check share the invalid-name fix (`host.firewall.rule`, the key the host file has). PR-Agent's "format replica" (tests compare against hand-built PowerShell output) ticketed as #601; site doc repeat → #593. The review job published nothing on its first two runs (NaN exhaustion), third run did.

## Verdict

`queue-empty`: every item in queue.order was attempted. Six merged to close their items (#376, #573, #500, #415, #561, #455 in three slices), plus #587 (HOST-005's AC6 fix). One parked: HOST-005-archive, which needs Manu's call on the release marker. No PR opened by this run is left open.

## Merged

| Item | PR(s) | Head |
|---|---|---|
| #573 | #590 | 1de2425 |
| HOST-005 AC6 | #587 | e67d994 |
| #376 | #597 | d77cc17 |
| #500 | #591 | 25a66fa |
| #415 | #586 | 51f2d8f |
| #455 | #594 (W1), #599 (W2), #600 (W3) | 2771d84, 5c42542, 9719bad |
| #561 | #596 | 74391be |

## Awaiting Manu

1. **#523 release (1.21.0)**: yours, untouched.
2. **HOST-005 archive / release marker**: #579 merged as plain `feat(host):`, so #523 files the breaking `host init` change under Features. (A) BEGIN_COMMIT_OVERRIDE on #579 and bump to 2.0.0, or (B) keep 1.21.0 with a prose note. Options are on #458. Round-2 `review.md` (FAIL, deepseek-v4-flash) is untracked in worktree chore-archive-host-005; its verification.md needs the AC6 correction from #587 before archive.
3. **test-macos runner starvation (#592)**: every PR today had test-macos cancelled after about 15 min queued with no runner. The limit looks like it lives in `.github/workflows/ci.yml`, so it was reported, not touched.
4. **Live Windows acceptance for #455**: unit tests and `test-windows` cover W1-W3; nothing has run the real PowerShell reads on a host (see #601).
5. **#583** (OPS-002, `.github`) and **#584** (dependabot `go.mod`): not mine, untouched.
6. **#428 / #420**: left open as instructed.

## Filed during the run (detect → ticket)

#588 (split runHostSetup), #589 (Merge drops the port-range derivation error), #592 (test-macos starvation), #593 (site getting-started still says `tailscale up --unattended`), #595 (auth_key_file ancestor walk), #598 (POSIX ACLs ignored on Unix ancestors), #601 (Windows PowerShell reads tested only against hand-built output).

## Evidence index

- Every merge went through `dotf pr land` after `dotf pr triage-queue --pr N` exited 0; each PR carries a `## Review triage` comment per reviewed head.
- Local gate per push: `go test -race ./...`, `golangci-lint run ./...` (native and `GOOS=windows` for host changes), `gosec ./...`, plus `scripts/check-*.sh` for doc changes.
- Mutant claims in specs were re-run before being written; one W1 claim was corrected (3→2 subtests) after it proved wrong.

## Budget and host

- Items: 7 of 7 attempted (max_items 7). No item hit the 3-round fix breaker; the churn was reviewer rounds after each master merge, not failing fixes.
- test-macos reruns: at most 2 per PR (#591), under the 3 limit; #600's review job was re-run twice for the same reason class (NaN).
- Host: `caffeinate` held PreventUserIdleSystemSleep for the whole window (checked at 22:05Z), so the window was covered.

## Next action

Decide HOST-005's release marker (A or B on #458), then archive HOST-005 with the round-2 review and the AC6 correction, and cut #523.
