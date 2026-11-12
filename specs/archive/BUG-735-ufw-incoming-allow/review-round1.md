---
spec: "BUG-735-ufw-incoming-allow"
verdict: "FAIL"
reviewed_sha: "39beef4674bd0c9dce0eea0e6d6015cad8fc254e"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: `BUG-735-ufw-incoming-allow` (issue #735), whole change at `39beef4`, diff range `42e34ea9d5fcbd3abfcff3e9aeaadf22773d9a1b...HEAD`.
All five commits in the range belong to this change (`a029d05` spec, `df17a28` unit test, `c8b5a57` fix, `35b258d` e2e, `39beef4` docs) — no unrelated work arrived through a merge, so nothing outside the spec was judged.
Contract digests match: `dotf spec digest BUG-735-ufw-incoming-allow` returns exactly the `contract_digests` in `review-request.json` (`proposal.md` `91cb91f3…`, `tasks.md` `863bbd8c…`, `features.json` `a81b9a35…`).

**Sources**: `specs/BUG-735-ufw-incoming-allow/{proposal,tasks,verification}.md`, `features.json`, `review-request.json`; `git diff 42e34ea…...HEAD` over `internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `scripts/e2e/linux.sh`, `docs/adr/adr-017-host-desired-state.md`, `docs/design/host-client-parity.md`, `docs/runbooks/guide-rdp-host-setup.md`.

**What I ran (fresh, this session)**

| Command | Result |
|---|---|
| `go build -o /tmp/tsb-bug735 ./cmd/ts-bridge/` | exit 0 |
| `go test ./internal/host/ -run 'TestUFWPolicy\|TestManualRewriteClearsSuccess\|TestIP6Policy' -count=1` | `ok ts-bridge/internal/host` |
| `go test ./internal/host/ -count=1 -timeout 5m` | `ok ts-bridge/internal/host` |
| `GOOS=linux go vet ./internal/host/`, `GOOS=windows go vet ./internal/host/` | exit 0 both |
| `gofmt -l internal/host/ scripts` | no output |
| `shellcheck -S warning scripts/e2e/linux.sh` | clean |
| Mutation A — delete `read.inputOpen = ipt.inputOpen` in `readUFW` | `TestUFWPolicy/incoming_allow:_both_policies_ACCEPT` FAILS (`fix lacks "`ufw allow <your SSH port>/tcp`"`) → red-green holds |
| Mutation B — restore `read.backend != backendIPT` in `withV6Policy` | `TestUFWPolicy` AC1 **and** AC2 subtests FAIL (`outcome ok success true, want manual=true`) → red-green holds |
| Probe (scratch test, deleted) — active ufw, its own allow decides, `iptables -S INPUT` exits 4 | `state=2 reason="" inputOpen=false outcome=ok firewall_ok=true` → see finding F3 |
| Probe (scratch test, deleted) — AC1 fixture, AC print | `check outcome=manual ok=false`, `setup outcome=manual success=false`; message text quoted in F4 |
| `dotf spec digest`, `dotf spec archive` pre-flights (scratch copy in `mktemp -d`) | digests match; archive refuses on promotion candidates — see F1 |

`make e2e-linux` was **not** run: it drives real VMs through `dotf vm` on a developer machine and is not available in this session, so AC4 stays UNVERIFIED by me. `golangci-lint`/`gosec` were not run (the repo makes the CI `lint`/`security` jobs authoritative); `gofmt`/`vet` stand in, as `AGENTS.md` directs.

### Spec and task alignment

- **Diff matches the proposal's "What"**: `readUFW` keeps the v4 kernel policy; `policyRead` (iptables **or** active ufw) replaces the three `backend == backendIPT` gates (`withPolicy`, `withV6Policy`, `linuxFirewallOK`); the ufw fix names `ufw allow <SSH>/tcp` then `ufw default deny incoming`, and `IPV6=yes` + `ufw reload` for v6; `setup` still only rewrites step text and never writes a policy (only `addScopedRule` writes, unchanged).
- **AC1/AC2/AC3 are covered by a named test and its red-green is real**: `TestUFWPolicy` fails under each of the two mutations above and passes on the change. That is stronger evidence than `verification.md` carries.
- **Deviations are documented, not silent**: reading the kernel policy instead of parsing `ufw status verbose` is argued in `proposal.md` ("Deviation from the issue's proposed fix"); the `IPV6=no` reboot-dependent outcome is stated as a host truth rather than a flake. Both are defensible.
- **No test was deleted.** `TestIP6Policy`'s ufw case was *narrowed* to `ufwInactive: true` (correct: that case's claim was that an inactive ufw's open policy is already reported), and the removed assertion's subject is now covered by `TestUFWPolicy/IPV6=no_after_a_boot`. Retention bar met.
- **Docs follow the tasks item**: ADR-017 Amendment B line, parity-doc firewall rows, and a runbook troubleshooting row all name #735 and match the code's fix text.
- **The spec's own bookkeeping is untouched template.** `proposal.md` AC boxes 0/4 ticked, `tasks.md` 0/8, `features.json` both features `state: "pending"` with empty `evidence`, `verification.md` still containing `<hash>`, `<command> -> <output / coverage %>` and unanswered promotion candidates. `status: implementing`.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | verification artifact | `verification.md` proves nothing: it is the scaffolded template (`Criterion 1 -> commit <hash> / test <name>`, `<command> -> <output>`, three unanswered promotion candidates), and `dotf spec archive` refuses on it today. | Observable: pre-flight in a `mktemp -d` copy with an otherwise passing `review.md` + `review-request.json` returns `Error: promotion candidates in verification.md are not all answered: Lesson … unanswered; ADR-worthy … unanswered; New pattern candidate … unanswered; no flag skips this check`. | UNTESTED — no test covers spec-artifact completeness; the archive pre-flight is the only gate, demonstrated refusing. | spec (`verification.md`; outside the contract set, safe to fix now) |
| Major | REAL | acceptance criterion | AC4 was never executed. The e2e case exists (`scripts/e2e/linux.sh:179-188`, commit `35b258d`) but no run is recorded anywhere, `tasks.md`'s `make e2e-linux` box is unticked, and the reviewer cannot run it (VM e2e, developer machine only). AC4 is therefore an unproven acceptance criterion, not a verified one. | `git show 35b258d` adds only the script; `verification.md` has no test-status line; repository doctrine states VM runs never happen in CI. | UNTESTED (by construction: `make e2e-linux`). | spec (`verification.md`) + a recorded developer-machine run, or a tracked ticket if it cannot be run |
| Major | REAL | resilience / correctness | On an **active ufw** the exposure signal silently degrades to "safe" when the `iptables` read itself fails: `readUFW` copies `ipt.inputOpen` but drops `ipt.reason`, so with a ufw allow deciding `rulePresent` the policy is never read and `check` still reports `ok` with `firewall_ok: true`. The raw iptables backend reports `unverified` on the same failure, so an active ufw is *less* safe than an iptables host — exactly the blindness #735 is about, left open on a path the proposal's Risks section denies ("a broken install, not a mode"; it names only a **missing** binary, not a failing read such as xtables-lock contention). Not a regression (before the change ufw reported no policy at all), but an unclosed hole in the feature that claims to close it. | Probe reproduced: active ufw + `ufw allow in on tailscale0 …` + `iptables -S INPUT` exit 4 → `state=rulePresent reason="" inputOpen=false outcome=ok firewall_ok=true`; the existing fixture `linux_steps_test.go:244` ("ufw host, a ufw allow decides first", `exit: {iptRead: 4}` → `rulePresent`) pins the same state. | UNTESTED — no test asserts any exposure outcome when `readIptables` errors on a ufw host. | code (`internal/host/linux_steps.go` `readUFW`) + tests |
| Minor | REAL | message clarity / docs | On an active ufw the step message still says "**the iptables INPUT policy is ACCEPT**, so unless another firewall filters this host every port is open…" while the same step's fix names ufw. On a ufw host the "unless another firewall filters this host" escape *is* the firewall being reported, and the runbook's #691 row is keyed on that exact sentence, so an operator (or a grep) can match the wrong troubleshooting row. | Probe output: `MSG=Firewall allows port 3389; the iptables INPUT policy is ACCEPT, so unless another firewall filters this host every port is open on every interface and the tailscale0 scope restricts nothing; …` beside `FIX=as root, allow SSH first (`ufw allow <your SSH port>/tcp`) …`. | `TestUFWPolicy` asserts the **fix** strings only; message wording is UNTESTED. | code (message text) + the runbook row key |
| Question / assumption | THEORETICAL | operator guidance | The ufw fix unconditionally tells the operator to run `ufw allow <your SSH port>/tcp` before closing the default. On a tailnet-only host that opens SSH on every interface, which the change's own doctrine ("scope everything to `tailscale0`") argues against. It matches the pre-existing iptables advice, so it is consistent rather than new — but ufw is where the scoped form is easy to write (`ufw allow in on tailscale0 to any port <SSH> proto tcp`). Confirm this is the intended wording, or scope it. | Code read of `withPolicy` (ufw branch), no reproduction of harm. | UNTESTED | code (fix text), follow-up |

No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags remain in the spec folder.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1–AC3 verified by a named test whose red-green I reproduced with two mutations; AC4 has no behavioural proof (unit simulation only), and F3 shows a path where the new signal reports safe while unread. |
| Verification       | C | The Go test is named and reproducible, but `verification.md` carries no evidence at all, both `features.json` entries are `pending`, and AC4 never ran. |
| Scope              | A | The diff is exactly the proposal's "What" plus the three docs the tasks item names; no creep, no opportunistic refactor (`iptPolicyFix` is a pure extraction). |
| Reliability        | C | Setup never writes a policy and no failure path can lock an operator out; but F3 leaves one error path both unhandled and silently optimistic. |
| Maintainability    | B | `policyRead` names the gate, `iptPolicyFix` de-hoists a literal, comments explain *why*; `withPolicy`/`withV6Policy` stay well under CC 10; the new table test is the right shape. |
| Handoff-readiness  | C | Implementation only: verification/tasks/AC/`features.json` untouched, so the next session inherits no recorded evidence and the archive is blocked. |

### Verdict
FAIL

- Path that decides it: three **REAL Major** findings (F1 verification artifact, F2 AC4 unproven, F3 silent `ok` on an active ufw whose policy read failed). The rubric (C in Verification and Reliability, no D) alone would put this at PASS WITH GAPS, but `severity × reality` dominates, and F3/F2 are not hypothetical.

### Recommended next steps

**Spec set — `verification.md` (outside the contract set; safe to edit now, no re-review cost):**
1. Fill the evidence table with the named tests actually in the change: AC1/AC2 → `TestUFWPolicy` (`incoming_allow:_both_policies_ACCEPT`, `IPV6=no_after_a_boot:_v6_ACCEPT_only`), AC3 → `TestUFWPolicy` (`incoming_deny:_both_policies_DROP`, the `inactive`/`unreadable` loop) plus `TestManualRewriteClearsSuccess`; record `go test ./internal/host/ -run 'TestUFWPolicy|TestManualRewriteClearsSuccess' -count=1` with its output.
2. Answer the three promotion candidates (`yes: <path>` / `no: <reason>`). `dotf spec archive` refuses until they are answered — demonstrated in this review, so it is the first thing the next session will hit.
3. AC4: run `make e2e-linux` on a developer machine and paste the five `ufw incoming …` PASS lines, or state in `verification.md` that it is deferred with a tracked ticket number. Do not leave it silent.
4. Set both `features.json` `state`/`evidence` entries.

**Code + tests — for the re-review (F3):** on an active ufw, when `readIptables` itself fails, keep `ipt.reason` instead of discarding it and make the exposure unread rather than absent (`unverified`/a note that clears `firewall_ok`), so an active ufw is never reported safer than an iptables host on the same failure. Add the fixture to `TestUFWPolicy`: active ufw, its own scoped allow, `iptables -S INPUT` exit 4 → assert the outcome is not `ok` and `linuxFirewallOK` is false. F4's message wording (and the runbook row key) can ride along or be ticketed.

**Contract set — only on a FAIL-resolution re-review:** `proposal.md` / `tasks.md` / `features.json` text edits invalidate this verdict. Ticking the `[ ]` boxes is normalised out of the contract digests, so ticking is safe; rewriting the Risks bullet to cover a *failing* (not merely missing) iptables read is a text edit and belongs in the re-review round.

**Do not archive in this state**: the gate refuses on `verification.md` today, AC4 is unproven, and F3 is a real false-safe report on the very mode the spec targets.
