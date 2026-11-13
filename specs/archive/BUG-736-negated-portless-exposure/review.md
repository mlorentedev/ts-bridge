---
spec: "BUG-736-negated-portless-exposure"
verdict: "PASS"
reviewed_sha: "dcea7d40b2921b964281f94d556dc15abdcf5e20"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: `BUG-736-negated-portless-exposure` — whole diff `git diff c337705bbd73abc0c0f76a3d40a41646bbbfd577...HEAD` at `dcea7d40b2921b964281f94d556dc15abdcf5e20` (5 first-parent commits; 3 are this spec's: `8c0c0a4`, `ddd7e21`, `dcea7d4`, plus `bc3de59` (its TDD red step) and `d923fbc` (`#755`, unrelated CI work that merged in). PR: `fix/issue-757-positive-sport-exposure` (#759, open, `Closes #757`).
**Sources**: `specs/BUG-736-negated-portless-exposure/{proposal,tasks,verification,features}.md`, `internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `scripts/e2e/linux.sh`, `docs/adr/adr-017-host-desired-state.md`, `docs/design/host-client-parity.md`, `docs/lessons/lesson-055-2026-11-04.md`, `harness/reviewer-pool.json`. Contract digests compared with `dotf spec digest BUG-736-negated-portless-exposure`: equal to `review-request.json` (features.json `023d2637…`, proposal.md `e053567c…`, tasks.md `f2fd09c3…`), so the contract is the one the launcher recorded.

### Spec and task alignment

- All `tasks.md` boxes are ticked; the diff contains each claimed artifact (predicate, step row, e2e cases, parity doc, ADR-017 amendment, lesson-055 update).
- The contract was changed in this range on purpose, per the round-4 disposition: proposal "What" now says a source-port match narrows nothing and AC3 lists the source-port-only allow as reported (`#757`, commit `dcea7d4`). Code, tests, ADR-017, parity doc and `features.json` f1 follow that wording. No drift found between the "What" bullet, AC1–AC4 and the diff.
- Round-4 Major (positive source-port allow read `ok`) is addressed: `iptExposes` routes a rule with no *destination* port match to `iptPortlessCovers`, and `iptPortlessToken` models `--sport`/`--sports` (negated or not) and `-m multiport` as narrowing nothing, while conntrack/state beside them still narrows.
- Residuals named in proposal Risks are real and ticketed open: `#743` (negated non-loopback scope; v4 port-bearing rules, and port-less rules on both reads), `#745` (`--dports 22,3389` fix text without the SSH caution), `#758` (replies-held allow naming a port still reported), `#753` (jumped-chain port-less accept, decided here as option A). I re-read each ticket: all OPEN except `#753` (OPEN) and `#736` (CLOSED by #742). Deferrals are therefore tracked, not verbal.

### What I verified by running (not by reading assertions)

All commands were run in this session in this worktree; scratch probes in `/tmp/probe736.VnuD` (copy of the tree), never in the repository.

| Check | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go test -race ./...` | every package `ok` (16 packages, incl. `internal/host`) |
| `go test ./internal/host/ -run 'TestReadIptablesUnscoped\|TestExposureShapes'` and `'TestIP6Exposure\|TestExposureShapes'` (`features.json` f1/f2 commands) | both `ok` |
| `golangci-lint run` (2.14.0, pinned) | `0 issues.` exit 0 |
| `GOOS=linux go vet ./...`, `GOOS=windows go vet ./...`, `gofmt -l .` | clean |
| `bash -n scripts/e2e/linux.sh`, `shellcheck scripts/e2e/linux.sh` | clean |
| Red-green, mutation 1: `iptExposes` reduced to `return iptPortCovers(f, port)` | red in `TestExposureShapes` (14 rows), `TestReadIptablesUnscoped/{a_port-less_accept_is_foreign,an_accept_for_every_port_but_SSH_is_foreign}`, `TestIP6Exposure/{a_port-less_v6_accept,a_v6_accept_for_every_port_but_SSH}`, `TestIP6Jumps/{a_jump_for_every_port_but_SSH,a_jump_for_every_source_port,a_port-less_accept_in_a_chain_is_reported}`, 6 `TestCheckForLinux` and 2 `TestSetupForLinux` rows |
| Mutation 2: drop the `--sport`/`--sports` token from `iptPortlessToken` (revert only `#757`) | red in `TestExposureShapes`, `TestCheckForLinux`, `TestIP6Jumps` |
| Mutation 3: revert `legacyHeld` to `state == rulePresent && …` (revert review-round-3 F1) | red in `TestSetupForLinux/…is_the_only_allow:_scoped_rule_added,_manual_(#736_review_round_3)` (outcomes `[ok changed ok]`) and `TestCheckForLinux/…is_the_only_allow:_manual_(#736_review_round_3)` |
| Mutation 4: drop the loopback branch of `iptOpenNegation` (revert review-round-2 F1) | red in `TestExposureShapes/{!_-i_lo,!_-i_lo+,!_-s_127.0.0.0/8}` and the `! -i lo` check row |
| Probe of 17 hand-built rule shapes through both reads | no false report and no missed shape outside the documented residuals; see below |

Probe results worth recording (v4 `iptUnscoped` / v6 `ip6Exposed`, port 3389): a rule naming the port together with a source port is still reported; a rule naming only other destination ports is not; `--sport 22 -m limit …` and `--sport 22 -m conntrack --ctstate NEW` are *not* reported (fails closed, as the proposal says for unmodelled narrowings) — the second is an admitted under-report, but it is the same fail-closed posture the port-less branch already had, and it is not a regression. `-A INPUT ! -s 10.0.0.0/8 -p tcp -j ACCEPT` is not reported by *either* read, which matches the verification residual ("`iptPortlessCovers` reads the negation as an unmodelled narrowing, so neither read reports it"); the v4/v6 asymmetry only applies to port-bearing rules, as documented. I list these as dispositions, not open findings: `#743` is open and the owner accepted the deferral.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | spec artifact accuracy | `verification.md` says "every pre-existing step row unchanged", but two pre-existing rows were changed on purpose: `TestIP6Exposure`'s "a port-less v6 accept is the operator's posture" (`OutcomeOK` → `OutcomeManual`) and `TestReadIptablesUnscoped`'s "a port-less accept is the operator's posture" (`0/0/true` → `0/1/false`). The "Decisions" section names the change; the "Test status" sentence contradicts it. | `git diff c337705…HEAD -- internal/host/linux_steps_test.go`; `verification.md` §Test status vs §Decisions | `TestIP6Exposure/a_port-less_v6_accept`, `TestReadIptablesUnscoped/a_port-less_accept_is_foreign` | verification.md |
| Minor | REAL | test traceability | AC1's `check` leg for two of its three v4 shapes (`-p tcp -j ACCEPT`, bare `-j ACCEPT`) has no step-level row; `TestCheckForLinux` pins only `! --dport 22`, while `TestSetupForLinux` has a `-j ACCEPT` row. The shared `state==ruleAbsent && foreign` path is pinned, so no behaviour is unproven, but the AC names no check-row proof for those shapes. | Step rows in the test diff; `TestCheckForLinux` table | Step-level check for `-j ACCEPT` / `-p tcp -j ACCEPT`: UNTESTED; read level `TestExposureShapes`, setup `TestSetupForLinux/iptables,_an_accept_for_every_port:…(#736)` | tests |
| Minor | REAL | contract wording | AC3's closing clause "existing step outcomes are unchanged" reads as refuted by the port-less v6 step row this change flips; it is only true if read as scoped to the shapes AC3 lists. | `proposal.md` AC3 vs test diff; `#757` section of `verification.md` | `TestIP6Exposure/a_port-less_v6_accept` | verification.md note — a contract edit is *not* requested (it would invalidate this verdict for no behavioural gain) |
| Minor | THEORETICAL | maintainability | `iptExcludes` keeps a disjunct that is unreachable from its only caller: `!slices.Contains(iptDestPortFlags, f[i])` is called only for a destination port flag, so it is always false. Harmless, but it reads as if a source-port branch still existed. | code read of `iptExcludes` / `iptExcludesOthers` | UNTESTED (no behavioural effect) | code (optional follow-up, not blocking) |
| Minor | REAL | PR artifact | PR #759's body is stale against the spec: "`make e2e-linux`: running; recorded in `verification.md` when done", while `verification.md` records 129/0 at `ddd7e21`. The PR also carries no `## Review triage` section/comment, which this repo's review-attestation gate looks for before merge. | `gh pr view 759 --json body` | n/a (process artifact) | PR body + PR comment (not a repo file) |
| Question | REAL | AC4 evidence reach | I could not reproduce `make e2e-linux` (needs `dotf vm` + a Lima Ubuntu guest and a VM-length run outside this review window). The new cases are syntactically valid and shellcheck-clean; the 15 `#736`/`#757` cases are unexecuted by me. | `verification.md` AC4 + `#757` section; `scripts/e2e/linux.sh` | e2e cases — **UNVERIFIED** here | no action if the owner accepts the recorded run; otherwise re-run at `dcea7d4` and paste the counts |

No Blocker, and no open REAL Major belonging to this change. `#743`'s v4 blind spot is a REAL exposure, but it is pre-existing, not widened by this diff, named in proposal Risks, restated in `verification.md`, and tracked by an OPEN ticket — so it is a disposition, not a finding against this change.

### Code-level checklist (diff)

Injection / secrets / auth: not applicable (local firewall rule parsing, no shell interpolation — every command is passed as an argv slice). Resilience: the predicates fail closed (an unmodelled narrowing reads as "not exposing"), the exposure reads never delete a rule, and `setup` still appends its own scoped rule before reporting `manual`; `legacyHeld` gained a `ruleAbsent` arm that only fires when a foreign allow exists, and `ruleBlocked`/`ruleUnknown`/inactive-ufw are tested before it, so no delete path widened. Quality: new functions are short (max ~15 lines), cyclomatic complexity stays inside the repo's `nolint:gocyclo` baseline that this diff does not extend, comments explain *why*. Performance: linear scans over rule output, unchanged asymptotics.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | Every acceptance criterion is exercised at both reads and at step level for the adversarial shapes; four independent mutations of the fix each turn named tests red; my 17-shape probe found no defect outside the ticketed residuals. |
| Verification       | B | Reproducible for unit/lint/vet (I re-ran all of them), but AC4 rests on a VM run I cannot reach from here and `features.json` f3's evidence is a pointer rather than a command output. |
| Scope              | A | The spec's diff matches the proposal (predicate + reads, step rows, e2e cases, parity doc, ADR-017, lesson-055); the unrelated `#755` CI work in the same range is a merge, not creep. |
| Reliability        | A | Fails closed on unmodelled narrowings, never deletes a foreign rule, appends the scoped rule before reporting `manual`, and re-runs converge without duplicating it (pinned by the setup row's call list + e2e idempotence claim). |
| Maintainability    | B | Clear naming, short functions, WHY-comments; one dead disjunct in `iptExcludes` and the long baseline `iptPortlessToken` keep it from exemplary. |
| Handoff-readiness  | A | ADR-017 and the parity doc amended, lesson-055 updated to "closed", residuals ticketed (`#743`, `#745`, `#753`, `#758`), PR body links the contract change to its round-4 finding. |

Aggregation: no D, no C → **PASS**.

### Verdict

**PASS** (adversarial). The round-4 Major and the two earlier real defect classes (round-2 `! -i lo` miss, round-3 "no rule found") are fixed with red-green evidence that I reproduced by mutation, the contract change is consistent across proposal, code, tests and docs, and the contract digests the launcher recorded still match.

`dotf spec archive` is **advisable** once this review is committed with the PR: the reviewer id `nan/deepseek-v4-flash` is a member of `harness/reviewer-pool.json`, `reviewed_sha` is the reviewed head, and the contract files are untouched by this review. Nothing in this round blocks the archive; the findings below are outside the contract set.

### Recommended next steps (all outside the contract set — `proposal.md`/`tasks.md`/`features.json` are closed by this verdict)

- `verification.md`: replace "every pre-existing step row unchanged" with a line naming the two rows the change deliberately flips (port-less v4 read and port-less v6 step), and note that AC3's "existing step outcomes" clause is read as scoped to the shapes it lists. Disposition each as applied.
- `tests` (not blocking): add a `TestCheckForLinux` row for the bare `-j ACCEPT` and `-p tcp -j ACCEPT` check leg so AC1's "check" half has a named proof per shape; today only `! --dport 22` has one at step level.
- `code` (optional, follow-up ticket): drop the unreachable first disjunct in `iptExcludes`, or say in its comment why it is kept.
- PR #759: replace the stale "`make e2e-linux`: running" line with the recorded 129/0 at `ddd7e21`, and post the `## Review triage` comment naming this review before merge (the repo's attestation gate reads that marker).
- If the owner does not accept the recorded e2e run as evidence, re-run `make e2e-linux` at `dcea7d4` and paste the counts into `verification.md`; otherwise record that AC4's evidence is the `ddd7e21` run and is not reproducible in the review window.
