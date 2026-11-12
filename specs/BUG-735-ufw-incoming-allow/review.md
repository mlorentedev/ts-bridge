---
spec: "BUG-735-ufw-incoming-allow"
verdict: "PASS"
reviewed_sha: "7e036d4e3dfe52ed00a00088cf4210ac212465d1"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: `BUG-735-ufw-incoming-allow` (issue #735), whole change at `7e036d4`, diff range
`42e34ea9d5fcbd3abfcff3e9aeaadf22773d9a1b...HEAD`. All ten first-parent commits in the range belong
to this change (`a029d05` spec, `df17a28`/`6e2da7a` tests, `c8b5a57`/`f66786a` fixes, `35b258d` e2e,
`39beef4` docs, `0eee04f`/`4664ba6`/`7e036d4` spec bookkeeping) — nothing unrelated arrived through a
merge, so nothing outside the spec was judged.
Contract digests match: `dotf spec digest BUG-735-ufw-incoming-allow` returns exactly the
`contract_digests` in `review-request.json` (`proposal.md` `e0ff3360…`, `tasks.md` `295dfbf0…`,
`features.json` `a81b9a35…`, `spec.md` empty). No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags remain.

**Sources**: `specs/BUG-735-ufw-incoming-allow/{proposal,tasks,verification}.md`, `features.json`,
`review-request.json`, `review-round1.md`; `git diff 42e34ea…...HEAD` over
`internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `scripts/e2e/linux.sh`,
`docs/adr/adr-017-host-desired-state.md`, `docs/design/host-client-parity.md`,
`docs/runbooks/guide-rdp-host-setup.md`.

**What I ran (fresh, this session)**

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go test -race -count=1 -timeout 5m ./internal/host/` | `ok ts-bridge/internal/host 1.302s` |
| `go test -count=1 -timeout 5m ./...` (17 packages) | every package `ok`, no failures |
| `go test -count=1 -v -run 'TestUFWPolicy$/…|TestUFWPolicyUnread|TestUFWPolicyMessage|TestManualRewriteClearsSuccess' ./internal/host/` | all three `TestUFWPolicy` subtests, `TestUFWPolicyUnread`, `TestUFWPolicyMessage`, `TestManualRewriteClearsSuccess` PASS by name |
| `golangci-lint run` (mise-pinned 2.14.0) | `0 issues.` |
| `gofmt -l internal/host/`, `GOOS=linux go vet ./internal/host/` | no output / exit 0 |
| `bash -n scripts/e2e/linux.sh`, `shellcheck -S warning scripts/e2e/linux.sh` | clean |
| Mutation A — `policyRead` reverted to `return fw.backend == backendIPT` | `TestUFWPolicy` AC1 **and** AC2 subtests FAIL (outcome `ok success true`, fixes empty, `linuxFirewallOK = true`), `TestUFWPolicyMessage` FAILS → red-green holds for the whole feature |
| Mutation B — drop `&& fw.policyUnread == ""` from `linuxFirewallOK` **and** the `read.policyUnread = ipt.reason` assignment | `TestUFWPolicyUnread` FAILS (`setup/check = ok`, `linuxFirewallOK = true with the policy unread`) → red-green holds for the round-1 F3 fix |
| Probe (scratch test, deleted) — active ufw + scoped allow, `iptables -S INPUT` **succeeds** as `-P INPUT ACCEPT`, `iptables -S ts-input` exits 4 | `inputOpen=true policyUnread="iptables -S ts-input: exit status 4"`, outcome `unverified`, message says the **policy** could not be read and the fix says run `iptables -S INPUT` → see F1 |
| Probe (scratch test, deleted) — inactive ufw + ACCEPT kernel policy, and active ufw with no port rule + ACCEPT policy | inactive: `manual` naming `ufw enable`, no ufw-policy note (unchanged, as AC3 requires); active+absent: `manual`, ufw wording + ufw fix on both `check` and `setup`, `firewall_ok=false` |

`make e2e-linux` was **not** run: it drives a real Ubuntu guest through `dotf vm` on a developer
machine and is not available in this session, so **AC4 stays UNVERIFIED by me** and its recorded run
(`103 pass, 0 fail` on `4664ba6`) is the implementer's evidence, not mine. The e2e block itself is
statically sound (assertions match the `fwstep`/`json` helper shapes, the default is restored before
the iptables section, `shellcheck` clean).

### Spec and task alignment

- **Diff matches the proposal's "What"**: `readUFW`/`withUFWIptables` keeps the v4 kernel policy on an
  active ufw; one gate `policyRead` (raw iptables **or** active ufw) replaces the three
  `backend == backendIPT` gates in `withPolicy`, `withV6Policy`, `linuxFirewallOK`; the v4 fix names
  `ufw allow <SSH>/tcp` then `ufw default deny incoming` and never a raw `-P`; the v6 fix names
  `IPV6=yes` + `ufw reload`; an inactive ufw keeps its own `manual` (probed, not just asserted).
- **Setup writes no policy**: the only new writes are message/fix text plus the pre-existing scoped
  `ufw allow`/`iptables -A`; no `ufw default` command exists in non-test code (grep), matching AC1's
  "never `iptables -P`" and the e2e's "leaves the default" assertion. `iptPolicyFix` is a pure
  literal extraction, so the iptables wording is provably unchanged.
- **AC1/AC2/AC3/AC5 are each proven by a named test whose red-green I reproduced** (Mutations A/B
  above) — stronger than `verification.md` claims for itself. AC4 is proven only by the implementer's
  e2e record (UNVERIFIED here).
- **Round-1 dispositions are real**: F1 → `verification.md` now carries per-AC evidence, commands and
  counts; F2 → e2e record added; F3 → `policyUnread` + `TestUFWPolicyUnread`, and I confirmed it is
  red-green; F4 → ufw-specific wording + `TestUFWPolicyMessage`; F5 declined with a recorded reason in
  `verification.md` (unscoped `ufw allow <SSH>/tcp` kept because a LAN-only operator would be locked
  out), which is a stated decision, not silence.
- **No test was deleted**; `TestIP6Policy`'s ufw case was narrowed to `ufwInactive: true` — correct,
  because that case's claim was "an inactive ufw is already reported off", and its old subject (a ufw
  host's open v6 policy) is now covered by `TestUFWPolicy/IPV6=no_after_a_boot`.
- **Docs follow the tasks item**: ADR-017 Amendment B, both parity-doc firewall rows, and a new runbook
  troubleshooting row all name #735 and quote the code's own wording, so the runbook key matches the
  message an operator actually sees (the round-1 F4 hazard).
- **Considered and accepted, not findings**: a ufw host without the `iptables` binary goes blind
  (`readUFW` returns before the policy read) — `proposal.md` Risks names it and ufw depends on iptables;
  a ufw with `IPV6=no` reports a reboot-dependent kernel v6 policy — stated as host truth in both
  `proposal.md` and the runbook row. Both are documented decisions with the safety direction intact.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | message clarity / resilience | On an active ufw the new `policyUnread` signal is fed by **any** iptables read failure, not only a policy-read failure. When `iptables -S INPUT` succeeds and shows `-P INPUT ACCEPT` but the `ts-input` walk fails, the step reports "the INPUT policy could not be read (iptables -S ts-input: …)" and tells the operator to run `iptables -S INPUT` — the command that just worked — while the known ACCEPT policy is downgraded from `manual` (with ufw's fix) to `unverified`. Outcome is still non-ok and `firewall_ok=false`, so the safety direction is right; the diagnosis and next action are wrong. | Probe reproduced in-process: `inputOpen=true`, `policyUnread="iptables -S ts-input: exit status 4"`, `msg="…the INPUT policy could not be read (iptables -S ts-input: exit status 4)…"`, `fix="run \`iptables -S INPUT\` as root to see why it fails…"`. Reachability on a real host rests on a transient xtables-lock/backend failure of the second read only, which I did not observe. | `TestUFWPolicyUnread` covers only the `iptables -S INPUT` failure (`exit: {iptRead: 4}`); the `iptChain`-failure-with-readable-policy path is **UNTESTED** | code (`withUFWIptables`/`withPolicyUnread`: distinguish "policy unread" from "rules unread") + tests |
| Minor | REAL | spec accuracy (contract set) | `tasks.md` ticks "Failing table test `TestUFWPolicy`; `TestManualRewriteClearsSuccess` gains the ufw rewrites", but `TestManualRewriteClearsSuccess` is byte-identical to the base — it holds only the three iptables cases. The ufw `Success=false` property it was meant to gain **is** asserted, inside `TestUFWPolicy` (`s.Success == tt.manual`), so the behavior is proven and only the bookkeeping is wrong. | `git diff 42e34ea...HEAD -- internal/host/linux_steps_test.go` shows changes at `TestIP6Policy` plus three appended tests, none in `TestManualRewriteClearsSuccess`; that func at HEAD lists `"v4 policy"`, `"v6 policy"`, `"v6 exposure"` only. | `TestUFWPolicy` (behaviour covered); the tick itself has no matching diff — **UNTESTED as written** | spec (`tasks.md`, contract set — record in `verification.md` or carry to the next spec round; do not edit under this verdict) |

No Blocker and no REAL Major. Round-1 findings are all dispositioned (applied or declined-with-reason);
F5's decline is a taste call the human already has a recorded reason for, so I do not re-litigate it.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1/AC2/AC3/AC5 pass by name and fail under both of my mutations; AC4 verified only by the implementer's recorded VM run; F1 is a wrong-but-safe diagnosis on one input-failure path. |
| Verification       | B | `verification.md` names each AC, its commit and its test, and the unit half is reproducible with commands I ran; the e2e half (AC4) cannot be reproduced from this session. |
| Scope              | A | The diff is exactly the proposal's "What" plus the three docs the tasks item names; `iptPolicyFix` is a pure extraction, the new test case is narrowed correctly, no opportunistic refactor. |
| Reliability        | B | Setup never writes a policy and no path can lock an operator out; the policy-unread path fails closed (`unverified`, `firewall_ok=false`), with F1 as a misleading message rather than a wrong outcome. |
| Maintainability    | B | `policyRead` names the gate and documents why ufw counts, comments explain *why*, `withPolicy`/`withV6Policy` stay far below CC 10, the table test is the right shape; `appendFix` is a small helper with one caller-pair. |
| Handoff-readiness  | B | Spec, `features.json` and docs are filled and the promotion candidates are answered; one `tasks.md` tick overstates the diff (F2). |

Rubric aggregation: no D and no C → **PASS**; the severity axis adds no Blocker and no REAL Major, so
it does not escalate.

### Verdict
PASS

`dotf spec archive` is **advisable** in this state once the contract digests above still hold: the
review is fresh for `7e036d4`, the verification artifact is filled with reproducible evidence, the
promotion candidates are answered, and the two open findings are Minors that do not gate.

### Recommended next steps

**Outside the contract set (safe to apply now; disposition each in `verification.md`):**
- F2 — add the line the tick implies: state that a ufw manual rewrite's `Success=false` is proven by
  `TestUFWPolicy` (`s.Success == tt.manual`) rather than by `TestManualRewriteClearsSuccess`, so the
  next reader does not look for coverage that was never added. Ticking/rewording is a contract edit only
  if `tasks.md` itself is touched — under this verdict it must not be.
- AC4 — paste the five `ufw incoming …` PASS lines (or point at the log) beside the `103 pass, 0 fail`
  line, so the only criterion this reviewer could not reproduce carries its own artifact.

**Code + tests (follow-up ticket, not an archive gate):**
- F1 — in `withUFWIptables`, set `policyUnread` only when the *policy* line itself was unread (the
  `iptables -S INPUT` failure), and let a ts-input/chain failure keep the existing unverified-rules
  behaviour with its own reason; then a readable `-P INPUT ACCEPT` still yields `manual` with ufw's fix.
  Add the fixture to `TestUFWPolicy`: active ufw, scoped allow, `iptRead` = `-P INPUT ACCEPT …`,
  `exit: {iptChain: 4}` → assert `manual` and the ufw fix, not "the INPUT policy could not be read".

**Not recommended:** re-running the adversarial review before archiving — the contract set is unchanged
since `review-request.json` recorded its digests, and an edit to `proposal.md`, `tasks.md` or
`features.json` would invalidate this verdict rather than strengthen it.
