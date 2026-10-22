---
spec: "BUG-034-linux-host-idempotent"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "1afc5543107c21cd45f29c2cafb71435537aa622"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-22"
---

## Adversarial review

**Scope**: `BUG-034-linux-host-idempotent` (issue mlorentedev/ts-bridge#454, epic #450), whole change
`d881023670240b0cc35716e899854e31fec669b4...HEAD` (6 commits, +1096/−102). Round 3: the earlier verdicts
(`review-round1.md` FAIL at `79492ef`, `review-round2.md` FAIL at `854b799`) are judged here only as part of
the whole, never as a delta.

**Sources**: `specs/BUG-034-linux-host-idempotent/{proposal,tasks,verification}.md`, `features.json`,
`review-request.json`; the diff over `internal/host/{linux_steps,linux_steps_test,step,steps_test}.go`,
`cmd/cli/{host.go,host_json_contract_test.go}`, `docs/design/host-client-parity.md`,
`docs/lessons/lesson-042-2026-10-22.md`, `docs/lessons/_index.md`; PR #477 (open, **draft**, head
`1afc554`, `test` job in progress). I did not read the gate's implementation (outside the repo) and did not
touch `proposal.md`, `tasks.md` or `features.json`.

**Contract digests**: `dotf spec digest BUG-034-linux-host-idempotent` at the start and again after all
probing — **identical** to `review-request.json` (`features.json 7559819d…`, `proposal.md e80f6304…`,
`tasks.md d7a06cc2…`). This review describes the current contract.

**Verification actually run in this session**
- `go build ./...` → exit 0. `go test -race -count=1 ./...` → every package `ok` (13 packages, 0 failures).
- `GOOS=linux|windows|darwin go vet ./...` → clean for all three. `golangci-lint run` (mise-pinned) → `0 issues`.
- All five `features.json` verification commands run literally → **PASS** each. Every entry is `state: pending`
  with empty `evidence`, so no agent-claimed `passing` (the gating rule in `tasks.md` holds).
- **Mutation 1** (`xrdpSetupStep` `x.enabled && x.active` → `x.enabled`): `TestSetupForLinux/xrdp_enabled,_not_running`
  red (`outcomes = [ok ok ok], want [changed ok ok]`). **Round 2's REAL Major is now pinned.**
- **Mutation 2** (`destPortIndex` ignoring `to`): `TestUFWAllows/source_and_destination_port` red.
- **Mutation 3** (comment not truncated): `TestUFWAllows/with_comment` red.
- **Mutation 4** (delete `firewallSetupStep`'s `rulePresent` short-circuit): `TestSetupForLinux/ufw,_converged`
  and `/iptables,_converged` red — "never `-A` when the rule is present" is pinned.
- **Mutation 5** (drop the `Elevated()` gate): `TestIptablesRule/not_root`, `TestSetupForLinux/not_root:…` and
  `TestCheckForLinux/not_root,_iptables_host` red — the elevation gate is load-bearing and pinned.
- All five mutations reverted (`git diff` on `linux_steps.go` empty afterwards; tree clean apart from this
  review and the launcher's files).
- **ufw 0.36.2 probe, `ubuntu:24.04`** (this session): `ufw allow log 4000/tcp` renders verbatim in
  `ufw show added`; `ufw allow in on tailscale0 3389/tcp` is *rejected* by ufw ("Wrong number of arguments"),
  so the shorthand-with-interface form the parser does not accept is not a real input; after
  `ufw allow log 3389/tcp` then `ufw allow 3389/tcp`, `show added` lists **only** `ufw allow 3389/tcp`, and a
  third identical call prints "Skipping adding existing rule" (exit 0). See F1.
- **Not verified by me**: the two-real-run `host setup` transcript claimed in `verification.md` §Test status
  (I reproduced the ufw read/write behaviour in a container, not the binary end-to-end), the privileged
  iptables `-C` exit-status probe (round 2 ran it; I re-ran only its unit-level pins via mutations 4/5), and
  PR #477's CI outcome (`test` still running, the `review` job **SKIPPED** — a notice, not a review) → **UNVERIFIED**.

### Spec and task alignment

- Every acceptance criterion now has a named, mutation-defensible test: AC1 `TestSetupForLinux/*,_converged`
  (mut. 4), AC2 `TestIptablesRule` + `/ufw,_nothing_configured` + `/iptables,_rule_missing` (mut. 4/5),
  AC3 `TestUFWAllows` (mut. 2/3), AC4 `TestXRDP` **and** the step-level `TestSetupForLinux/xrdp_enabled,_not_running`
  + `TestCheckForLinux/xrdp_enabled,_not_running` (mut. 1 — round 2's gap), AC5 `TestSetupForLinux/not_root:…`,
  `/iptables_-C_fails_with_another_status`, `/no_systemctl` and `TestCheckForLinux/*` (mut. 5). No `[x]` in
  `tasks.md` lacks diff evidence.
- AC1's idempotence holds for every form the tool itself writes: iptables `-C`→0 after `-A`; ufw `show added`
  lists the rule `ufw allow <port>/tcp` writes. The residual exceptions are F1/F2 (operator-authored forms the
  read does not recognise) and F3 (duplicates left by the *pre-fix* code, never removed).
- AC5 holds and is enforced before any firewall command; check runs no mutating argv (`TestCheckForLinux`
  asserts it explicitly, the only `run` is the read-only `-C`, which ADR-017 §2 sanctions).
- Spec vs code: the proposal's "What" (comments, lists, ranges, source-port exclusion, elevation, xrdp service
  state, legacy-key consequences) now matches the shipped code — round 2's contract drift is closed. The only
  remaining gap in that paragraph is F1's `log` modifier, which the proposal's classifier (`f[1] == allow` plus
  the `out`/`route`/`deny` exclusions) does not anticipate.
- Scope matches the proposal: production changes stay inside `internal/host` plus two help-text lines; the
  `Runner.Elevated()` seam addition, the `xrdp detection`→`xrdp service` rename and the `cfg.FirewallRule`
  message removal are all declared in `verification.md` §Decisions, and ADR-017 §3 pins `outcome`, not `name`.
  `proposal.md` is still `status: implementing` and the task Closing boxes are unticked — correct pre-merge.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | ufw read (AC3) | A rule carrying ufw's `log` / `log-all` modifier is read as **absent**. `ufwAllows` requires `len(f[2:]) == 1` for the shorthand, so `ufw allow log 3389/tcp` falls through to `destPortIndex` and finds no `port` keyword. AC3's own sentence ("a rule counts when it is an `allow` on `<port>/tcp`") is therefore not met for a real ufw line, and the exclusions the proposal does name (`out`, `route`, `deny`, source port) do not cover it. Consequence is bounded, not a loop: setup appends `ufw allow 3389/tcp`, which ufw applies as a *replacement* (the logging flag is lost) and reports `changed` once; the next run converges. `host check` reports drift on a converged host once. | `ufw show added` verbatim, ufw 0.36.2 in `ubuntu:24.04`, this session: `ufw allow log 4000/tcp`. `ufwAllows("ufw allow log 3389/tcp", 3389)` → `false` (throwaway in-package test, deleted; tree clean). Follow-on measured: after `ufw allow 3389/tcp`, `show added` lists only the plain rule and a repeat call prints "Skipping adding existing rule". | **UNTESTED** (no `log`/`log-all` case in `TestUFWAllows`) | code (`ufwRuleMatchesPort`: skip a leading `log`/`log-all` token in the shorthand) + tests |
| Minor | THEORETICAL | ufw read (AC3) | A source-only allow (`ufw allow from 100.64.0.0/10`, no destination port) is read as absent: `destPortIndex` returns −1 when `from` is present and `to` is not. The port *is* already allowed (by a broader rule), so setup appends a narrower `3389/tcp` rule and check reports one-time drift. No exposure is widened — the appended rule is narrower than the one already present — which is why this does not rise above Minor. | Parser probe this session: `ufwAllows("ufw allow from 100.64.0.0/10", 3389)` → `false`. Operator-visible impact not measured (the container chain short-circuited before this rule was created). | **UNTESTED** | code (treat a `from`-only allow as covering the port, or document it) + tests; can ride #479 |
| Minor | REAL | legacy state (AC1, docs) | Hosts that ran the **pre-fix** code keep the duplicates it created, and nothing detects, reports or removes them: `iptables -A` ran unconditionally on every setup, so N runs left N identical `INPUT` rules. After this change `iptables -C` sees the first duplicate and reports `ok`, so the drift is silent (check cannot distinguish one rule from five). "Idempotence" here means "adds no *new* duplicate", which is what AC1 says, but the operator is left with a host ADR-017's model would call converged while the ruleset is not. | `git show d881023:internal/host/linux_steps.go` → `configureLinuxFirewall` runs `ufw route allow …` / `iptables -A …` with no read, on every run. No message or check reports the count. | **UNTESTED** (no test asserts what happens with pre-existing duplicates) | docs/spec note + follow-up ticket (a counting read is out of this PR's scope) |
| Minor | REAL | handoff record | `verification.md` §Test deletions records the retention-bar *mapping* (removed case → successor → "retained?") but omits three of the five fields the bar requires: the failure each removed case could detect, a **shown** mutation that makes a surviving named test red, and the focused command that validates the remainder. The successors are genuine (mut. 4 shows it), so the deletion itself is sound — the record is what is thin. | `verification.md` §Test deletions vs the retention bar; round 2 recorded the `git log -S` history only in its review, not in the spec. | `TestSetupForLinux/ufw,_converged` (mutation 4, this session) | spec (`verification.md` — outside the contract set, so it cannot stale this verdict) |
| Minor | REAL | doc drift | `internal/host/host.go` still comments `Service.Active` as "nil until a step reads the service state (#454, #455)". On Linux the step now *does* read it (`checkLinux` sets `Active` whenever `x.readable`), so the comment describes the previous state of the code it sits next to. | code read of `internal/host/host.go` (`Service` struct) vs `checkLinux`. | `TestCheckForLinux/converged` (pins the non-nil value, not the comment) | code (one-line comment) |
| Question / assumption | THEORETICAL | archive pre-flight | `specs/BUG-034-linux-host-idempotent/review-transcript.jsonl` — the launcher's own untracked/ignored transcript artifact — contains 27 literal occurrences of `[AGENT-DRAFT]` (the skill text this run read is echoed into it). If `dotf spec archive`'s drafts pre-flight greps the *folder* rather than tracked `*.md`, the archive will refuse and blame the reviewed spec for a file the review tooling wrote. | `grep -c "AGENT-DRAFT" specs/BUG-034-linux-host-idempotent/review-transcript.jsonl` → 27; file is gitignored (absent from `git status`, absent from `git ls-files`). | n/a | tooling (outside this repo) or delete/relocate the transcript before archiving — **UNVERIFIED**, the gate's implementation is out of scope |

### Test deletions

The five Linux cases removed from `internal/host/steps_test.go` (`TestSetupFor` ×2, `TestCheckFor` ×3) guarded
the behaviour this spec replaces; one of them ("linux with ufw") asserted the *defect* (`run ufw route allow …`),
so removing it was mandatory. Successors are stricter and are shown, not assumed:

| Removed case | Successor | Shown by |
|---|---|---|
| `TestSetupFor/linux with ufw` | `TestSetupForLinux/ufw, converged` | mutation 4 turns it red (`outcomes = [ok changed ok]`) |
| `TestSetupFor/linux without ufw or xrdp, Tailscale down` | `TestSetupForLinux/xrdp missing, no systemctl, no backend, Tailscale down` | pins the added no-backend branch in the golden argv |
| `TestCheckFor/linux, port found in ufw` | `TestCheckForLinux/converged` | mutation 5 (`not root, iptables host`) + `converged` pin the same outcome through the new read |
| `TestCheckFor/linux, firewall unreadable without root` | `TestCheckForLinux/not root, iptables host` | mutation 5 turns it red |
| `TestCheckFor/linux, nothing` | no exact Linux successor; all-zero legacy keys with Tailscale down asserted by `TestCheckFor/windows, registry unreadable and no rule` | ADR-017 §3 derivation is OS-independent; the Linux reads it depended on are covered by `TestCheckForLinux/xrdp not installed` |

No seam lost a production caller; `git log -S` (round 2) dates the removed cases to `f3dbc8d`/`d881023`, i.e.
before this spec. No retained test was weakened. F4 is about the **record**, not the decision.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All five ACs verified and mutation-backed; one measured ufw form (`log` modifier) still reads absent, and its consequence is a bounded one-time drift rather than a loop. |
| Verification       | B | Named test per AC, every command reproduces here, mutations red where claimed; the end-to-end two-run transcript is claimed in `verification.md` and I reproduced only its parts (container ufw behaviour, not the binary). |
| Scope              | A | Diff matches the proposal; the only additions beyond it (`Runner.Elevated()`, the step rename, the dropped `cfg.FirewallRule` in messages) are each recorded in `verification.md` §Decisions. |
| Reliability        | B | Errors, unexpected exit statuses and non-root all land on `unverified`/`failed` with no mutation; idempotent for every rule form the tool writes, partial for operator-authored forms. |
| Maintainability    | A | Small functions, `ufwRuleMatchesPort`/`portListHas`/`destPortIndex` each single-purpose and table-tested, `protoTCP` de-duplicated, comments explain why (the non-root `-C` exit-1 hazard). One stale comment (F5). |
| Handoff-readiness  | B | Triad, `lesson-042`, `_index` and the parity doc updated in the diff; promotion lines answered; PR #477 open against the reviewed sha. Deletion record thin (F4). |

Aggregation: all dimensions B or above, no **D**, no **C** → mechanically **PASS**; no REAL Major and no
Blocker, so nothing forces below it. I record **PASS WITH GAPS** rather than PASS because AC3's letter is
measurably unmet for one rule form (F1) and four gaps need a disposition.

### Verdict

**PASS WITH GAPS** — no Blocker, no REAL Major. Round 2's REAL Major (the untested "xrdp enabled but not
running" step branch, i.e. L4 itself) is closed and mutation-pinned; the remaining gaps are minors, all
tracked with a fix location.

### Recommended next steps

The **contract set is closed** by this verdict: do not edit `proposal.md`, `tasks.md` or `features.json`
(any edit stales this review and reopens the round). Disposition each item in `verification.md` —
applied, ticketed, or declined with a reason — or carry it into a follow-up ticket.

1. **Code + tests (F1, the only correctness gap).** Add the verbatim `ufw allow log 3389/tcp` line to
   `TestUFWAllows` and make `ufwRuleMatchesPort` skip a leading `log`/`log-all` token in the shorthand branch;
   `UFWAllows` must then be `true` and a `-A`-free `TestSetupForLinux` case must stay `ok`. Both files are
   outside the contract set.
2. **Tests / #479 (F2).** Either count a `from`-only allow as covering the port, or add it to #479's
   out-of-scope note so the next reader does not rediscover it. Do not ask for a proposal edit on this PASS.
3. **Ticket + doc note (F3).** File the pre-fix duplicate rules as a follow-up (`host check` could report the
   duplicate count from `iptables -S`), and note in `verification.md` that idempotence starts from a host the
   tool writes, not one the old code already duplicated.
4. **`verification.md` (F4).** Complete the deletion record: what each removed case could detect, the mutation
   above that turns its successor red, and the focused `go test ./internal/host -run '…'` command.
5. **One-line comment (F5).** `internal/host/host.go`, `Service.Active`.
6. **Before archiving (F6, UNVERIFIED).** If `dotf spec archive` refuses on `[AGENT-DRAFT]`, delete or relocate
   `review-transcript.jsonl` first (it is gitignored, so nothing is lost) rather than reaching for
   `--force-with-drafts`.
7. **Already tracked.** #478 (xrdp `LookPath` / `static` units) verified OPEN, #479 verified OPEN, PR #477 open
   and **draft** at the reviewed sha; the `pr-agent/review` job shows `SKIPPED`, which is a notice, not a
   review — it does not satisfy the independence requirement and must not be counted as one.

### Archive advice

`dotf spec archive` is **advisable** in this state: the verdict is passing, the contract digests are unchanged
since the launcher recorded them, no `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain in the tracked spec
files, and all three promotion lines are answered (`lesson-042` exists). Two things to do first, neither
blocking the verdict: mark PR #477 ready for review (a draft is not a reviewed change), and resolve F6 if the
pre-flight complains. `proposal.md` is still `status: implementing` and the tasks Closing boxes unticked —
both are the archive's own job.
