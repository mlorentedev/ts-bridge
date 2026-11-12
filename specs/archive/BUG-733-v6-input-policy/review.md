---
spec: "BUG-733-v6-input-policy"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "025a8690837eed2fc7c6d06877d40df82d1d87c6"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: BUG-733-v6-input-policy — `git diff 46b921c1a5c914f517582599bbda05c30386d59b...HEAD`
(6 commits, all this spec's work: `6a3e05b` spec, `b0abdea` test, `318627f` code,
`a9b06a3` test, `8814617` code+e2e, `025a869` docs).
**Sources**: `specs/BUG-733-v6-input-policy/{proposal,tasks,verification}.md`, `features.json`,
`dotf spec digest BUG-733-v6-input-policy`; diff over `internal/host/linux_steps.go`,
`internal/host/linux_steps_test.go`, `scripts/e2e/linux.sh`, `docs/adr/adr-017-host-desired-state.md`,
`docs/design/host-client-parity.md`, `docs/runbooks/guide-rdp-host-setup.md`.

Contract freshness: `dotf spec digest BUG-733-v6-input-policy` returns
`proposal.md cb401939…, tasks.md 53ac9696…, features.json 33005d59…`, byte-identical to
`review-request.json` — the contract I reviewed is the contract the launcher recorded.

### Spec and task alignment

Per criterion, against the diff (not against the author's claim):

- **AC1** (v4 `DROP` + v6 `ACCEPT`: `manual`, message names the ip6tables policy, fix names
  `ip6tables -P INPUT DROP` with the keep-open rules, `firewall_ok` false, setup runs no `-P`) —
  **met**. `readIP6` now returns `iptPolicyAccept(string(out))` (`linux_steps.go:243`);
  `withV6Policy` (`:305-321`) appends the note, rewrites `ok`/`changed` to `manual` with the fix; a
  step whose outcome is `unverified`/`failed` keeps its own fix and gains only the message
  (`:309-311`); `linuxFirewallOK` folds `v6Open` in (`:274`). "Setup runs no `-P`" is held by both
  the code (nothing but step text is written) and the test's `strings.Contains(c, " -P ")` scan of
  the recording runner's calls. Proven by `TestIP6Policy` rows `v4_DROP,_v6_ACCEPT,_rule_present`
  and `…_rule_absent:_setup_adds_it_and_reports_manual`.
- **AC2** (v6 `DROP`, failed v6 read, no `ip6tables`, ufw backend, unverified v4 step unchanged; a
  failed/unverified step gains only the note) — **met**. `readIP6` returns `false` on `LookPath`
  failure and on the error path (`:236-239`); `withV6Policy` bails on `backend != backendIPT`
  (`:306`), held by the ufw assertion at the end of `TestIP6Policy`; `TestIP6Exposure` keeps the
  `v6_read_fails` and `no_ip6tables` rows; `TestPermissiveInputPolicy` and
  `TestIP6Policy/an_unfollowable_v4_jump_keeps_its_outcome` cover the unverified/failed v4 step.
- **AC3** (v4 and v6 both `ACCEPT`: one `manual` step whose fix names both) — **met**. Composition is
  `withIP6(read, withPolicy(read, …), port)` (`:1082`, `:1267`), so v4 rewrites first and the v6 fix
  is appended to it; `TestIP6Policy/both_ACCEPT:_one_manual_step_naming_both_policies` asserts both
  `iptables -P INPUT DROP` and `ip6tables -P INPUT DROP` in the single step.
- **AC4** (e2e: check `manual` + `firewall_ok` false under a permissive v6 policy; `ok` once the
  guest closes it) — **present in the script, UNVERIFIED here**. `scripts/e2e/linux.sh:196-207` adds
  the four cases, then closes v6 with loopback, replies, ICMPv6 and SSH first. `make e2e-linux` needs
  a throwaway VM (`dotf vm`) that a review run does not have; the case ordering is otherwise sound
  (the `#526` v6-exposure cases at `:251-262` follow the close, so they still read a `DROP` policy).

Test-traceability (whole change, run fresh in this session):

```
$ go test ./... -count=1 -timeout 5m      → exit 0, 17 packages ok, 0 FAIL
$ go test ./internal/host/ -run 'TestIP6Policy|TestIP6Exposure|TestPermissiveInputPolicy|TestReadIP6NoteQuotesTheTool' -count=1 -v
                                          → PASS (TestIP6Policy 6 subtests, TestIP6Exposure 10, TestPermissiveInputPolicy 7, TestReadIP6NoteQuotesTheTool)
$ gofmt -l .                              → (no output)
$ go vet ./...                            → exit 0
$ shellcheck -s bash scripts/e2e/linux.sh → (no output)
$ bash scripts/tests/test-e2e-{headscale,mesh-preflight,probe,report,run-teardown}.sh
                                          → PASS ×5 (the CI `hygiene` shim suite for the e2e harness)
```

Red-green, by mutation, reverted after each (`git checkout -- internal/host/linux_steps.go`;
tree verified clean):

| Mutation | Result |
|---|---|
| `readIP6` returns `false` instead of `iptPolicyAccept(...)` | `TestIP6Policy` FAILS on 4 rows: `setup = ok "Firewall allows TCP 3389", want manual`, `linuxFirewallOK = true, want false` |
| `withV6Policy` drops its `unverified`/`failed` guard | `TestIP6Policy/an_unfollowable_v4_jump_keeps_its_outcome` FAILS: `setup = manual …, want failed`, fix gains the v6 DROP |
| `withV6Policy` drops the ufw backend guard | `TestIP6Policy` FAILS: `a ufw host's step changed` |

So the three behaviours AC1–AC3 assert are pinned by tests that go red when the behaviour goes away —
not by tests that merely run.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | spec / verification artifacts | `verification.md` is an untouched template (literal `<hash>`, `<command>`, three empty `-` decision slots), every task and every AC checkbox is still `[ ]`, `features.json` f1/f2 are `state: pending` with empty `evidence`, and the three promotion lines are unanswered. The spec cannot archive in this state: `dotf spec archive` refuses unanswered promotion lines, and this repo's activation rule requires `verification.md` to carry evidence before a review is even proposed. The behaviour *is* tested; what is missing is the recorded criteria→evidence mapping the archive gate reads. | `specs/BUG-733-v6-input-policy/verification.md` (whole file), `tasks.md` (all `- [ ]`), `proposal.md` ACs `[ ]`, `features.json` (both `"state": "pending"`, `"evidence": ""`) | named coverage exists and I ran it: `TestIP6Policy`, `TestIP6Exposure`, `TestPermissiveInputPolicy` — the gap is the artifact, not the coverage | spec artifacts. `verification.md`, task/AC checkbox ticks and `features.json` `state`/`evidence` are all excluded from the digest normalisation, so closing this **does not** invalidate this review |
| Minor | REAL | spec drift | `proposal.md` names the v6 keep-open rules as "loopback, replies and SSH"; the code, ADR-017's Amendment B line, the runbook row and the test also require `ip6tables -A INPUT -p ipv6-icmp -j ACCEPT`. The proposal is the only place that still describes the pre-`a9b06a3` fix. AC1's wording ("with the keep-open rules") is satisfied by the code, so this is stale prose, not wrong behaviour — and the safer behaviour is the one shipped. | `proposal.md` §What bullet 2 and AC1 vs `linux_steps.go:313-315`; `docs/adr/adr-017-host-desired-state.md:296` ("with ICMPv6 kept open first because neighbor discovery needs it") | `TestIP6Policy` keep-list asserts the ICMPv6 rule (same substring in fix and assertion) | spec artifacts — record it under `verification.md` → "Decisions made during implementation". Do **not** edit `proposal.md` now: that would invalidate this verdict |
| Minor | REAL | code consistency | `withV6Policy` rewrites the `Fix` of an already-`manual` step, while `withPolicy` (v4, #691) only appends to its `Message`. On a v4-`ACCEPT` host that already has a `manual` step for another reason (e.g. a foreign unscoped v4 rule, `linux_steps.go:1077-1079`), the step gains the v6 `-P DROP` fix but **not** the v4 one — the operator is told to close v6 and left free to leave v4 open, though the message does name the v4 policy. AC1–AC3 only require `ok`/`changed` steps, so this is outside the contract. Reproduced with a temporary in-package probe (`withIP6(read{inputOpen:true, v6Open:true, backend:iptables}, withPolicy(read, SetupStep{Outcome:manual, …}), 3389)`), deleted afterwards: `has v4 DROP=false has v6 DROP=true`. | probe run in this session (output above); guards at `:295` vs `:309` | UNTESTED — no row combines a `manual` v4 step with a v6 `ACCEPT` policy | code + tests (add a row) — or decline in `verification.md` with a reason; the v4 half is #691's shape, so a follow-up ticket is defensible |
| Minor | THEORETICAL | false positive on nft-legacy hosts | When the legacy `iptables`/`ip6tables` binaries are installed while tailscaled filters through nftables, `ip6tables -S INPUT` reports the empty legacy chain with `-P INPUT ACCEPT`, so a correctly filtered host reads `manual` with `firewall_ok: false` on every run and `setup` never converges. `readLinuxFirewallV4` consults `tsInputInNFT` only when `iptables` is *absent* (`:358-368`), so the legacy-binary-plus-nft case is not detected on the v4 path either — this is the pre-existing shape of the #526 exposure scan and #691's v4 read, not new. The fix text names the dismissal ("or confirm another firewall filters this host"), and nftables detection is declared out of scope (#739). | `linux_steps.go:358-368`, `tsInputInNFT` (`:540-554`); no test covers a legacy-binary + nft-mode host | UNTESTED | code/tests — follow-up ticket, not this spec |
| Question | SPECULATIVE | e2e robustness | The new block's first assertion greps `ip6tables -S INPUT` for a `-P INPUT ACCEPT` line. On a guest with IPv6 disabled at boot or without `ip6tables`, that command fails and AC4's case fails at its first line rather than reporting the misconfiguration. The proposal's Risks name the guest's v6 policy as `ACCEPT`, so this is an assumption to confirm on the VM, not a defect. | `scripts/e2e/linux.sh:196-198` | `make e2e-linux` (not run in this review) | tests — confirm on the guest |

Notes that are not findings:

- The `ip6Captured` → `ip6Closed` rewrite inside `TestIP6Exposure` narrows that table to the
  rules-only scan; the `ACCEPT`-policy expectation it used to carry moved to `TestIP6Policy`
  (policy-on path) and the exposure scan is policy-independent (`ip6Exposed` never reads the policy
  line, `:251-259`). No coverage was lost; mutation 1 above shows the policy behaviour is still
  pinned. Cosmetic only: the row still named "captured chain" now feeds a derived fixture
  (`ip6Closed`), which the const comment explains.
- No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain in any spec file.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1–AC3 each met and each pinned by a test that fails when the behaviour is mutated away, and the negative paths (ufw, no `ip6tables`, failed write, unverified/failed step, forged comment line) are covered — but one observed defect outside the criteria: on an already-`manual` step the v4 fix is omitted while the v6 one is appended (finding 3). |
| Verification       | C | `verification.md` is an unfilled template with no criteria→evidence mapping, no test-status line and no answered promotion lines, so the proof exists in the suite but not in the spec. |
| Scope              | A | Diff is the firewall v6-policy read, its tests, one e2e block and the three doc rows the tasks named; nothing unrelated. |
| Reliability        | B | Error paths handled and `setup` provably writes no policy; the residual is the nft-legacy false positive, pre-existing and out of scope. |
| Maintainability    | A | `gofmt`/`go vet` clean; `withV6Policy` is ~20 lines, flat, ≤ 2 branches; the shared `iptPolicyAccept` is reused rather than duplicated and its first-line rule is documented. |
| Handoff-readiness  | C | ADR/parity/runbook lines are updated, but the spec's own evidence, task ticks, feature states and promotion answers are missing, so the archive gate will refuse as it stands. |

### Verdict
PASS WITH GAPS

The behaviour is correct and independently reproducible; what remains open are spec-artifact gaps
(two C dimensions), two REAL minors (one of them reproduced), one THEORETICAL minor and one
question. No Blocker, no REAL Major in code, no D.

### Recommended next steps

All of these are **outside the contract set's digest** (`verification.md`, checkbox ticks,
`features.json` `state`/`evidence`), so applying them keeps this verdict fresh — none requires a
re-review round.

1. Fill `specs/BUG-733-v6-input-policy/verification.md` with the mapping I verified:
   AC1 → `TestIP6Policy` rows `v4_DROP,_v6_ACCEPT,_rule_present` and
   `v4_DROP,_v6_ACCEPT,_rule_absent:_setup_adds_it_and_reports_manual`
   (`go test ./internal/host/ -run 'TestIP6Policy/v4_DROP,_v6_ACCEPT' -count=1 -v`);
   AC2 → `TestIP6Policy/an_unfollowable_v4_jump_keeps_its_outcome` plus the ufw assertion at the end
   of that test, `TestIP6Exposure/v6_read_fails` and `TestIP6Exposure/no_ip6tables`; AC3 →
   `TestIP6Policy/both_ACCEPT:_one_manual_step_naming_both_policies`;
   AC4 → `scripts/e2e/linux.sh:196-207` **plus the `make e2e-linux` output** — that run is still owed
   and is the only criterion this review could not reproduce.
   Test-status line: `go test ./... -count=1 -timeout 5m` → exit 0, 17 packages ok;
   `gofmt -l .` and `go vet ./...` clean.
2. In "Decisions made during implementation", record the two decisions the diff shows but the
   proposal does not: ICMPv6 added to the keep-open rules (`a9b06a3`/`8814617`), and the
   `ip6Captured` → `ip6Closed` fixture split in `TestIP6Exposure`.
3. Answer the three promotion lines. Suggested, with a reason each: Lesson `yes:` a short entry in
   `docs/lessons.md` (an IPv6 permissive policy is invisible to an exposure scan — the policy line,
   not the rules, decided this bug); ADR `no: ADR-017 Amendment B already carries the line`;
   Pattern `no: single project`.
4. Tick the implementation and closing boxes in `tasks.md` and the four AC boxes in `proposal.md`
   (checkbox state is excluded from `dotf spec digest`, so the verdict survives), and set
   `features.json` f1/f2 to `done` with their evidence once `make e2e-linux` has run.
5. Disposition the rest in `verification.md` — apply, ticket, or decline with a reason: the
   `manual`-step asymmetry (finding 3) and the nft-legacy false positive (finding 4). Both are
   pre-existing shapes; tickets rather than fixes in this spec are reasonable.

`dotf spec archive` is **not** advisable until steps 1–4 are done: the gate refuses an unanswered
promotion line and a `pending` feature, and step 4's evidence is what the Definition of Done's
Evidence check asks for.
