# Ledger — ts-bridge-overnight-2026-11-02

Armed 2026-11-02T16:05:53Z, expires 2026-11-02T22:11:38Z. Cold start on 9374ed7: dotf doctor 21/21, go test ./... green.

| time (UTC) | item | state | evidence |
|---|---|---|---|
| 16:12 | #541 #543 | triage updated (master merges, no new findings); queue exit 0 | PR comments |
| 16:13 | #543 | landed by `dotf pr land` | master 49400f0 |
| 16:20 | #508 | PR #547 opened (ready) | go test -race 13 ok; golangci-lint 0 |
| 16:33 | #538 | landed (base merged first) | master 09403cb |
| 16:40 | #539 | landed | master be6d66a |
| 16:13 | #541 | first land refused (re-review after --update-branch); triaged; re-landing | — |
| 16:44 | #402 | PR #548 opened (ready) | mutants M1/M2 FAIL, 400 stress runs ok, 13 pkgs ok, lint 0 |
| 16:48 | #401 | criterion 1 PARKED (A refuse / B wizard keeps key), board Blocked; criteria 2-4 in PR #549 (Refs) | 4 RED/GREEN rows in PR body; 13 pkgs ok; lint 0 |
| 17:05 | #541 | merged by the operator (land had refused: test-macos cancelled + re-review) | master 3683300 |
| 17:10 | #547 | CodeRabbit Minor (IPv6 zone) applied b2b6ad2; triaged | 13 pkgs ok |
| 17:12 | #548 | PR-Agent one-sided bound: comment narrowed a2050ff; triaged | ok |
| 17:15 | #549 | PR-Agent: per-source glob + connect/browser prefix applied 4db3cda; 2 declined with reasons | wrapper mutants FAIL; 13 pkgs ok |
| 17:18 | #498 | PR #550 opened | check-attr unspecified→lf; 13 pkgs ok; lint 0 |
| 17:27 | #547 #550 | re-review findings applied (0db1664 zone in no-port fallback; 29476f2/00e5217 CR byte check + working recovery hint); #548 #549 re-triaged | 13 pkgs ok; CRLF mutant FAIL |
| 17:27 | #505 | PR #551 opened | 5 fixtures, RED first; bash+zsh OK; shellcheck clean |
| 17:30 | #547 #550 | lander re-reviews: #547 clean (finding resolved); #550 vacuous-skip finding applied (skip only without .git; GIT_DIR mutant now FAIL) | ok |
| 17:31 | #427 | PR #552 opened: persist-credentials read only from the step's with: (block + flow) | 4 fixtures RED first, control OK; bash+zsh; real tree 13/13; shellcheck clean |
| 17:33 | #403 | PR #553 (Refs) two-page pagination fixture; criteria 2-5 PARKED (A split / B one ticket), board Blocked | 2 pagination mutants FAIL |
| 17:47 | #547 | MERGED under the grant at 0db1664; #508 board Done | lander |
| 17:50 | #549 | smoke red: bats asserted the removed "read auth key file" prefix; now asserts "stat auth key file" (0a0f159) | bats 54/56, 2 Linux-only host cases fail on macOS as before |
| 17:50 | #551 | PR-Agent fixture finding applied (go-version-file added); old-depth + disabled-rule mutants FAIL | bash+zsh OK |
| 17:50 | #548 #550 | re-reviews repeated already-applied findings; triaged | — |
| 17:50 | #548 | MERGED under the grant at aecb2f7; #402 board Done | lander |
| 18:07 | #549 | MERGED under the grant at a26e67f (Refs #401; #401 stays Blocked on the --target decision) | lander |
| 18:09 | #552 | PR-Agent: blank-line finding not reproduced (fixture added); nested flow finding RED then fixed (dcbcdf8) | 33 fixtures x bash+zsh; real tree 13/13 |
| 18:09 | #553 | triaged (no major issues; criteria 2-5 parked on #403) | hygiene green |
| 18:23 | #551 | MERGED under the grant at d80a313; #505 board Done | lander |
| 18:25 | #550 | CodeRabbit: skip only on ErrNotExist, fail other stat errors (9fa613d) | go test ok; lint 0 |
| 18:40 | #553 | PR-Agent one-sided fixture: mirror two-pages-marker-first added | .[-1] and .[0] mutants FAIL |
| 18:40 | #499 | PR #555 opened: go flags, go.exe, full go help list; wrappers/other actions/IFS/GOTOOLCHAIN declared out of scope in header (advisor-checked scope fork) | 4 fixtures RED first + control; 4 mutants FAIL; bash+zsh OK |
| 18:40 | budget | 8 of 8 items attempted (508 402 401 498 505 427 403 499); #530 #526 #466 not started | — |
| 18:51 | #550 | MERGED under the grant at bb57d0d; #498 board Done | lander |
| 19:05 | #553 | MERGED under the grant at 88a424e (Refs #403; #403 stays Blocked) | lander |
| 19:10 | #555 | PR-Agent: toolchain premise softened to verified facts (b045e1e) | go.mod toolchain=0; GOTOOLCHAIN in .github=0 |
| 19:19 | #555 | PR-Agent: bug/doc/telemetry fixtures added | 3 mutants FAIL; bash+zsh OK |
| 19:19 | #552 | blank-line finding declined again (hygiene green on cce518c); nested-flow false red declined (fail-closed, by design) | hygiene success |
| 19:35 | #555 | MERGED under the grant at d0e4c8b; #499 board Done | lander |
| 19:40 | #552 | PR-Agent security note (flow with: under env bypass) RED then fixed (8694942, brace depth) | fixtures bash+zsh; real 13/13 |
| 19:52 | #552 | review check failed: PR-Agent action timeout (notice, not review); branch update reruns it | — |
