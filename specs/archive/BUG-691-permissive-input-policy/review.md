---
spec: "BUG-691-permissive-input-policy"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "afe45f601490e32a31cc66b4973f3b702bf04899"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-11"
---

## Adversarial review

**Scope**: `BUG-691-permissive-input-policy` — `git diff 949ba4bc202099520eafb88064ca7403d0d19483...HEAD`
(6 branch commits: `556c49b`, `9a50f3d`, `81f6a80`, `2abafba`, `c2f25a5`, `afe45f6`, plus one merge of
`origin/master` at `40bc4af` that brought #714's v6/nftables read — judged only where it interacts).
Contract digests from `review-request.json` match `dotf spec digest BUG-691-permissive-input-policy` exactly
(`3c14ea90…`, `b4d37d05…`, `aa964993…`); the contract set is unmodified since the request.

**Sources**: `specs/BUG-691-permissive-input-policy/{proposal,tasks,verification,features}.md`,
`internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `scripts/e2e/linux.sh`,
`docs/design/host-client-parity.md`, `docs/adr/adr-017-host-desired-state.md`, `cmd/cli/host.go`.

### Spec and task alignment

- **AC1** (ACCEPT + scoped rule → `manual` in check and setup, message names the policy, fix names a
  DROP policy, `firewall_ok` false) — met. `withPolicy` promotes `ok`/`changed` to `manual`,
  `linuxFirewallOK` carries the `inputOpen` term. Named test `TestPermissiveInputPolicy/ACCEPT,_rule_present`,
  run green this session.
- **AC2** (ACCEPT + rule absent → setup appends the scoped rule, reports `manual`, runs no `-P`) — met.
  Row `ACCEPT, rule absent: setup adds it and still reports manual` asserts `run iptables -A INPUT …`
  and that no recorded call contains `" -P "`; `iptables -P` appears nowhere in non-test code or scripts
  (`grep`). Green this session.
- **AC3** (DROP / ufw backend / already `unverified`/`failed` keep their outcomes) — met for DROP and for
  the unfollowable-jump row (`check=unverified`, `setup=failed`). The ufw half is structural, not a table
  row: `readUFW` never copies `inputOpen` onto the merged read, and `withPolicy`/`linuxFirewallOK` both
  gate on `backend == backendIPT`. Confirmed by code read; see finding F4 for what that structure also
  hides.
- **AC4** (e2e on the Linux VM) — **UNVERIFIED at review time**: the VM runs never happen in CI, no e2e
  log is committed, and reproducing requires `dotf vm` on a developer machine. What I could check:
  `shellcheck -S warning` clean on the three `scripts/e2e/*.sh` touched, and the new block's ordering is
  coherent (ACCEPT case, `iptables -D`, then loopback/ESTABLISHED/22, then `-P INPUT DROP`, then the
  existing scope cases that assume a closed policy).
- **tasks.md** — every box ticked, and each tick maps to a commit in the range (`9a50f3d` red test,
  `81f6a80` fix, `2abafba` e2e, `c2f25a5` parity doc, `afe45f6` evidence). `#733` ("read the IPv6 INPUT
  policy") exists and is OPEN; PR #734 exists. No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain.
- **TDD order** — the red state is reproducible, not just asserted: removing `withPolicy` from both
  wrappers (`mutation B`) makes `TestPermissiveInputPolicy` fail on outcomes, message and fix; removing the
  `inputOpen` term from `linuxFirewallOK` (`mutation A`) fails on `linuxFirewallOK = true, want false`.
  Both mutations were applied and reverted; `git status` is clean apart from the spec folder.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | input parsing | `iptPolicyAccept` scans **every line** of `iptables -S INPUT` for the literal `-P INPUT ACCEPT`, not the first line as its own doc comment claims. A multi-line `--comment` is printed raw by iptables (the codebase already models this in `TestReadIptablesMultilineComment`), so a comment containing a newline plus that text makes a **DROP** host report `manual` and `firewall_ok=false`. Fail-safe direction, but it is a permanent false `manual` (and `host check` exit 2) on a correctly configured host. | Reproduced this session with a throwaway in-package probe (deleted afterwards): fixture `-P INPUT DROP` + `-m comment --comment "x\n-P INPUT ACCEPT\ny"` + the scoped rule → `iptPolicyAccept = true`, `read.inputOpen = true`, `firewallSetupStep` = `manual` ("…the INPUT policy is ACCEPT…"), `linuxFirewallOK = false`. Correct fixture (`-P INPUT ACCEPT` with no such comment) behaves per AC1. | UNTESTED — `TestReadIptablesMultilineComment` pins the rule path only; no row asserts comment lines cannot forge the policy line. | code (`iptPolicyAccept` should read the line it documents, or the policy from the comment-stripped parse) + tests (a row in `TestReadIptablesMultilineComment`-style table) |
| Major | THEORETICAL | scope of the fix | The proposal declares ufw hosts out of scope because "an active one sets a DROP-equivalent policy". That is an assumption, not a guarantee: ufw with `DEFAULT_INPUT_POLICY="ACCEPT"` leaves `-P INPUT ACCEPT`, i.e. exactly the #691 blindness, and the same class of bug is then reported converged on a ufw host. `linuxFirewallOK` and `withPolicy` cannot see it by construction (`inputOpen` is never copied in `readUFW`). | Code read of `readUFW` (copies `copies`/`foreign`/`scoped`/`copiesAdmit`, never `inputOpen`) and of the proposal's Out-of-scope bullet. Not reproduced on a host; no fixture exists. | UNTESTED | spec (confirm the assumption, or ticket the ufw-side follow-up) — contract-set edit, so a follow-up round, not an edit under this verdict; `verification.md` can record the disposition now |
| Minor | REAL | docs / ADR consistency | ADR-017 Amendment B still states that "under `iptables -P INPUT ACCEPT` the scope restricts nothing and `check` does not say so (#691)". After this change `check` does say so; the ADR now documents the defect as open. `verification.md` records "ADR-worthy? no: it extends ADR-017's `manual` model without changing it" — true of the model, but the sentence it leaves behind is now wrong. | `docs/adr/adr-017-host-desired-state.md:295` (unchanged by the diff); the behaviour it contradicts is `linuxFirewallOK`/`withPolicy`. | UNTESTED (doc claim) | docs — an addendum line under Amendment B; outside the contract set, can be applied now |
| Minor | REAL | docs / spec claim | `proposal.md` Risks says "the runbook and parity doc say so". The parity doc does; **no runbook mentions the Linux firewall at all** — `grep -rn iptables docs/runbooks/` returns nothing, so an operator who hits the new `manual` has nowhere in the runbooks to read why. | `grep -rn "iptables" docs/runbooks/` → empty; the only `policy`/`manual` hits in `guide-rdp-host-setup.md` are the Windows Group-Policy case (lines 63-64). | UNTESTED (doc claim) | docs — a Linux firewall subsection in `docs/runbooks/guide-rdp-host-setup.md` (or a dedicated Linux runbook); `proposal.md` itself is contract-set, so record the disposition in `verification.md` |
| Minor | REAL | user-visible contract | Every default-ACCEPT iptables host that used to exit 0 now exits **2** from `host check` (a `manual` step is "not ok"), i.e. a scripted/CI `host check` starts failing on hosts nobody changed. Correct per ADR-017 §2 and intended per the proposal, but it is an operational consequence stated only in the proposal's Risks, not in README/runbook. | `cmd/cli/host.go:382-394` (`checkConvergence`: any step `!= ok` → `exitNotConverged`); new `manual` outcomes from `withPolicy`. | UNTESTED (no test asserts the check exit status on an ACCEPT-policy host) | docs (runbook/README note) + tests (optional: a `cmd/cli` test that `host check` exits 2 on an ACCEPT fixture) |
| Minor | THEORETICAL | test coverage | The fix commit moved 28 pre-existing step fixtures from `-P INPUT ACCEPT` to `-P INPUT DROP` (expected outcomes unchanged, which is right), which leaves the ACCEPT branch covered only by the five `TestPermissiveInputPolicy` rows. The composition of `withPolicy` with the other outcome paths — an unscoped *foreign* allow, a legacy copy, a diverted/unfollowable jump reaching `changed` — has no ACCEPT row, so a future change to those paths plus the policy promotion is unguarded together. | `git show 81f6a80 -- internal/host/linux_steps_test.go` (fixture move); `TestPermissiveInputPolicy` rows: present / absent / unverified only. | UNTESTED | tests |

Not findings, recorded so the next round does not re-litigate them: the v6 additions (`readIP6`,
`ip6Exposed`, `withIP6`, `toolSaid`) and the `scripts/e2e/tsfw_linux.sh` capture tooling arrived through the
`40bc4af` merge (#714), not from this spec; the `-P INPUT` bug and its `manual` outcome were decided by the
owner on #691, so the outcome choice is not re-opened here.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1-AC3 verified green and mutation-proved this session; AC4 not reproducible here; one reproduced false positive on a forged comment line. |
| Verification       | B | Named tests, commit SHAs and replayable commands; the e2e log is the author's and is not committed, so AC4 cannot be replayed by a reviewer. |
| Scope              | B | The spec's own six commits are tight and on-target; the range also carries #714 through a merge, which is not this spec's creep. |
| Reliability        | B | Error/unknown paths unchanged and fail-closed; `setup` never touches the policy (asserted); the forged-comment case over-reports rather than under-reports. |
| Maintainability    | B | Lint clean (`golangci-lint run ./internal/host/...` → 0 issues), gofmt clean, `GOOS=linux go vet` clean, functions well under 40 lines; the `iptPolicyAccept` doc comment misdescribes its own implementation (F1). |
| Handoff-readiness  | B | `verification.md` carries evidence and records the decisions; but ADR-017 now contradicts the shipped behaviour (F3) and a claimed runbook statement does not exist (F4). |

### Verdict
**PASS WITH GAPS**

No Blocker and no REAL Major: the acceptance criteria hold against a real test run and against two
mutations that turn the suite red. The reproduced defect (F1) over-reports on a correctly configured
host and has no security impact; it is a Minor and is UNTESTED. F2 is a THEORETICAL Major about the
scope the proposal declares out, so it is tracked with a disposition rather than gating. Rubric has no
D and no C.

### Recommended next steps

Contract set (`proposal.md`, `tasks.md`, `features.json`) — **closed by this verdict; do not edit**.
Disposition these in `verification.md` (excluded from the staleness check) or carry them into follow-up
tickets:

- **F1 (code + tests, apply in scope if a round is opened anyway)**: make `iptPolicyAccept` read the line
  its comment describes, or take the policy from the comment-stripped parse, and add a table row asserting
  a `-m comment` that spells `-P INPUT ACCEPT` does not forge the policy. If not fixed now, file it: a
  DROP host with such a comment reports `manual` and `firewall_ok=false` forever.
- **F2 (spec, follow-up round)**: confirm whether `DEFAULT_INPUT_POLICY="ACCEPT"` (a ufw host that is
  active and still leaves `-P INPUT ACCEPT`) is genuinely out of scope; if the assumption is wrong, it is
  the same bug behind a different backend and deserves its own ticket.
- **F3 (docs, apply now)**: add the closing line to ADR-017 Amendment B — `check` reports a permissive
  INPUT policy as `manual` since #691.
- **F4 (docs, apply now)**: give the runbook the Linux firewall entry the proposal claims exists, or
  correct the claim in the next contract round.
- **F5 (docs, optional)**: state the `host check` exit-2 consequence on default-ACCEPT hosts where an
  operator meets it (README/runbook), and consider a `cmd/cli` test for it.
- **F6 (tests, optional)**: one ACCEPT-policy row exercising a foreign/legacy allow, so the promotion
  path is guarded in combination with the pre-existing reads.
- **AC4**: keep the `make e2e-linux` log with the PR (or paste the pass/fail counts into
  `verification.md`) so the next reviewer can check it without a VM.
