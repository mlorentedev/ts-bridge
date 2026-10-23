---
spec: "BUG-481-firewall-deny-order"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "a466a8b0caf40df47979a568f5225809962798f4"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-22"
---

## Adversarial review

**Scope**: BUG-481-firewall-deny-order — `git diff 5572d4c4b800b035549b977f1d7a80480a897dda...HEAD` (single commit `a466a8b`, 9 files, +377/−27; production diff is `internal/host/linux_steps.go` only).
**Sources**: `specs/BUG-481-firewall-deny-order/{proposal,tasks,verification,features}.{md,json}`; `internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`; `docs/lessons/lesson-047-2026-10-22.md`, `docs/design/host-client-parity.md`; captured output of ufw 0.36.2 and iptables 1.8.10 in `ubuntu:24.04`.

### Evidence I produced (not the author's claims)

| Check | Command | Result |
|---|---|---|
| Contract freshness | `dotf spec digest BUG-481-firewall-deny-order` vs `review-request.json` | all three digests identical — review is not stale |
| Suite | `go test -race -count=1 ./...` | every package `ok` |
| Vet | `GOOS=linux\|windows\|darwin go vet ./...` | clean |
| Lint | `golangci-lint run` | `0 issues.` |
| Red-green, 8 mutations | one mutation each, then revert (`linux_steps.go`) | **8/8 red**: setup blocked-branch removal → `TestSetupForLinux/{ufw,iptables}`; check blocked-branch → `TestCheckForLinux`; DROP scope-guard removal → `TestFirewallDecideOrder/hardened`; `iptScopeFlags` narrowed → same; ufw `from`/`on` guard removed → same; ufw comment truncation removed → `deny_with_a_comment_naming_an_interface`; ufw `reject` case removed → `reject_before_allow`; `-j` portless token reverted → `TestIptablesDecideOnly/{blanket,tcp_reset}` |
| Real printed forms → production parsers | captured ufw/iptables output in `ubuntu:24.04` (`--cap-add NET_ADMIN`), fed verbatim to `ufwDecide`/`iptablesDecide` | `deny 3389`+`allow 3389/tcp` → blocked; `deny from any to any port 3389` prints as `ufw deny 3389` → blocked; `deny 3389/tcp comment 'deny from lan on purpose'` prints verbatim → blocked (the `comment` truncation is load-bearing: `from`/`on` inside the comment would otherwise read as scope); hardened two-rule listing → present; real `iptables -S INPUT` mixture → blocked, scoped-only → present. `iptables -A INPUT -4 -j DROP` normalises to `-A INPUT -j DROP` |
| End-to-end, built binary | `GOOS=linux` binary in `ubuntu:24.04`, ufw denied-first / iptables DROP-no-allow | ufw: `host setup` → `manual`, message names `ufw deny 3389`, exit 0, `ufw show added` byte-identical afterwards. iptables: `host setup` → `manual` naming `-A INPUT -p tcp -m tcp --dport 3389 -j DROP`, **nothing appended** (`-S INPUT` unchanged), `host check` → exit 2, `firewall_ok: false`. Both reproduce `verification.md`'s claims |
| features.json f2 non-vacuous | `go test ./internal/host/ -run 'TestFirewallDecideOrder/hardened' -v` | subtest actually runs (`--- PASS: …/hardened:_scoped_deny,_then_scoped_allow`), not a silent no-match |
| gosec | `gosec ./...` | **UNVERIFIED** — not installed here; the CI `security` job is the evidence. The diff adds no exec, no path, no input trust: it parses rule text already returned by `Run`-free reads |

### Spec and task alignment

- All five implementation tasks are ticked and every `[x]` is backed by named code/tests I found and re-ran; the only open box is "PR opened", which is expected at this phase.
- AC1 → met and verified end-to-end by me on both backends (`blocked`/`manual`, names the rule, no command run, `firewall_ok` false, check exit 2). AC2 → met (`hardened` row, `interface-scoped` row, plus a real hardened listing). AC3 → met except one shape (F1). AC4 → met: `TestFirewallDecideOrder` feeds one table to both backends with one expected answer.
- Non-goals respected: `-P INPUT DROP` still does not decide; `ts-input`/other chains ignored; scoped denies never decide; no ip6tables/nftables/firewalld code.
- Scope matches the proposal exactly: one production file, one test file, lesson 047, one design-table row. No creep, no deleted tests (the diff only adds cases).
- No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags in the spec files (checked; the only grep hits are in the untracked review transcript).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | spec-vs-code (AC3 wording) + coverage | A **port-bearing** iptables deny narrowed by an unmodeled match *does* decide: `-A INPUT -p tcp --dport 3389 -m conntrack --ctstate INVALID -j DROP` (and the `-m limit` variant) before an allow returns `ruleBlocked`. AC3's bullet says "an iptables deny narrowed by an unmodeled match … does not decide". The code is *parity-consistent* — a `--dport`-bearing ACCEPT with the same extra match reads `present`, because `iptPortCovers` ignores non-port matches — and the "What" definition ("unscoped when it would cover the port if it were an allow … no `-i`/`-s`/`-d`/`!` qualifier") is satisfied. So the operative definition and the allow-parity clause support the code; only the AC3 bullet, read alone, over-claims. Consequence: a false `manual` and `check` exit 2 on a host whose RDP port is genuinely open — an alarm in the safe direction, priced in `proposal.md` §Risks ("a `manual` note on a working host"). Rated Minor, not Major, because the ruleset shape is uncommon and the divergence is bounded by a declared risk; a reviewer who reads AC3's bullet literally would grade this Major and re-review. | Reproduced with a temporary probe test (removed): `--dport 3389 -m conntrack --ctstate INVALID -j DROP` + allow → state `ruleBlocked`; `-m limit --limit 5/min` variant → `ruleBlocked`. Reverting to "denies obey the portless all-tokens-modeled bar" breaks no test — the shape is unpinned | UNTESTED for this shape (`TestIptablesDecideOnly/conntrack_drop_does_not_decide` is portless only) | tests (pin whichever behaviour is intended); note it in `verification.md`. AC3's wording is a contract-set edit: **do not** apply it in this round — ticket it for the next contract touch |
| Minor | THEORETICAL | verification premise (ufw) | The ufw fix rests on premises about `ufw show added` that are asserted from one measurement and mostly hold, but one I could not reproduce: an `ufw insert 1` rule's position in `show added`. I *did* reproduce the others: `deny 3389` + `allow 3389/tcp` coexist in that order, `deny from any to any port 3389` folds to `ufw deny 3389`, `deny proto tcp to any port 3389` folds to `ufw deny 3389/tcp`, a comment is printed verbatim, and an identical match replaces in place (`Rules updated`). If `insert`-ordered rules were listed out of evaluation order, the ufw path could stop at the wrong first rule — the same class of defect #481 is about, just moved. | Container runs in `ubuntu:24.04` (ufw 0.36.2); the `insert 1` case did not survive a clean reset, so it is unconfirmed | UNTESTED (no case exercises an inserted rule) | tests + `verification.md` note: capture a real `insert 1` listing as a fixture, or record the premise as unverified |
| Minor | REAL | test coverage of AC1's "naming the rule" | No test asserts that the step message contains the blocked rule, and `blockedBy` is asserted only for iptables (`TestIptablesDecideOnly`) — the ufw rule string is never checked, and neither `firewallBlockedStep` message nor its fix text is asserted. The behaviour is correct (I verified both messages end-to-end), so this is a missing regression guard, not a defect. | `grep` of `linux_steps_test.go`: no `Message`/`blocks TCP` assertion; setup/check tables compare `calls`, `outcomes`/`steps` names and `Services` only | UNTESTED | tests (assert message contains the rule in the two setup rows and the check row) |

**Question / assumption (not gating)**: `ufw limit 3389/tcp` (a rate-limited *allow*) is skipped, so such a host reads `absent` and setup appends a plain allow after the limit rule. This is pre-existing and consistent with the proposal's "route, out and limit rules never decide" — flagged only because the new ordering logic is the natural place to notice it.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | Every AC verified including my own end-to-end run on both backends; one AC3 shape (port-bearing deny with an unmodeled match) is uncovered and unpinned. |
| Verification       | A | Commands + outputs are listed and all reproduced here (suite, vet, lint, mutation red-green, container e2e); only gosec was unreachable locally. |
| Scope              | A | Production diff is one function family in one file; docs and lesson are the placement-rule obligations, not creep; no test deleted. |
| Reliability        | A | Blocked path runs no mutating command (mutation-proven), re-runs are stable, error/unknown paths keep their prior outcomes, operator state is never reordered or deleted. |
| Maintainability    | B | All new funcs < 40 lines, CC ≤ 7, comments explain why (scope = operator judgment); `iptablesCovers`/`ufwAllows` now have no production caller (kept as bool wrappers per tasks.md, alive only for older tests). |
| Handoff-readiness  | A | Lesson 047 written and indexed, `docs/design/host-client-parity.md` row updated, `verification.md` filled; no deferred knowledge. |

### Verdict

**PASS WITH GAPS** — no Blocker, no REAL Major, no rubric D. Three minors tracked below; none changes the behaviour the spec exists to fix, which I reproduced end-to-end on real ufw and real iptables.

### Recommended next steps

All of these are **outside the contract set** (`proposal.md`, `tasks.md`, `features.json`), so none of them invalidates this verdict. Disposition each in `verification.md`.

- **tests** — add one row to `TestIptablesDecideOnly` (or `TestFirewallDecideOrder`) pinning `-A INPUT -p tcp --dport 3389 -m conntrack --ctstate INVALID -j DROP` before an allow, so F1's behaviour cannot flip silently; whichever answer is chosen, say so in `verification.md`.
- **tests** — assert the blocked-step message contains the deciding rule for ufw as well as iptables (`firewallBlockedStep`), closing F3.
- **verification.md** — record two unverified premises: `ufw insert 1` listing order (F2) and gosec (run only in CI).
- **ticket (not this round)** — clarify AC3 so the port-bearing, unmodeled-match shape is stated rather than inferred, and decide `ufw limit` (Question above).
- Archive state: `dotf spec archive` is admissible from the review gate's side (fresh passing verdict, contract digests unchanged). The spec's own closing box "PR opened referencing this spec folder" is still unticked — archive after the PR exists.
