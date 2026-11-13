---
spec: "BUG-762-v4-operator-chains"
verdict: "PASS"
reviewed_sha: "afea4e96156a2db0a23da47eba5ddc48eb5594d0"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: `BUG-762-v4-operator-chains` — the whole change, `git diff f8ace2d22e7b22b89ba6261cb04a57563a4fea2e...HEAD` (7 first-parent commits: `9414418` fix, `6819422` docs, `51d753f` round-1 fix, `0b8ca47`/`c4d8077`/`a4e0f41`/`afea4e9` spec), reviewed at `afea4e9`. Round 1 (`review-round1.md`, FAIL at `a4e0f41`) found one REAL Blocker and one REAL Major; both fixes are re-tested here as part of the whole change, not as a delta.
**Sources**: `specs/BUG-762-v4-operator-chains/{proposal,tasks,verification,features.json}`; `internal/host/linux_steps.go`; `internal/host/linux_steps_test.go`; `scripts/e2e/linux.sh`; `docs/design/host-client-parity.md`; `docs/runbooks/guide-rdp-host-setup.md`.

Evidence produced in this session (fresh, in this worktree at `afea4e9`):

- `go build ./...` → exit 0.
- `go test -race -count=1 ./...` → every package `ok`, 0 failures (18 packages).
- `golangci-lint run` (2.14.0, the `mise.toml` pin) → `0 issues.`; `gofmt -l .` empty; `go vet ./...` clean and also clean under `GOOS=linux`, `GOOS=windows`, `GOOS=darwin`.
- `dotf spec digest BUG-762-v4-operator-chains` → `proposal.md 38998be1…`, `tasks.md cc7d0166…`, `features.json 928e5c07…` — all three identical to `review-request.json`, so this verdict measures the contract as launched.
- `features.json` replayed as written: `go test ./internal/host/ -run 'TestCheckForLinux|TestSetupForLinux' -count=1` → ok; `go test ./internal/host/ -run 'TestCheckForLinux/chain|TestSetupForLinux/chain' -count=1 -v` → the six `#762`/`#762 review` chain rows run and pass.
- Mutation probes, run on a throwaway `git archive` copy under `mktemp -d` (repository untouched, `git status` shows only this file and the launcher's `review-request.json`):
  - drop `read.chainUnread = ipt.chainUnread` from `withUFWIptables` → the two ufw-host rows red (`ufw_host,_an_operator_chain_that_cannot_be_read…`, `ufw,_the_scoped_allow_in_place_and_an_operator_chain…`), i.e. round 1's Blocker has a named regression test that works.
  - drop `&& v != "0.0.0.0/0"` from `iptNarrowed` → `iptables,_a_jump_from_any_address_is_a_jump_for_every_packet:_manual_(#762_review)` red.
  - replace `!partlyUnread(fw)` with `fw.policyUnread == ""` in `linuxFirewallOK` → 2 rows red.
  - remove `chain == tsInputChain` from the v4 skip → 85 failing test lines; remove the `ufw-` skip → 22 (the `ufw-` count matches `verification.md`; the `ts-input` count does not, see M1).
  - scratch white-box probes (also on the copy, deleted with it): `-d 0.0.0.0/0` jump → chain followed and allow reported; `-s 0.0.0.0/0` **on the allow inside** the chain → `foreign=[]` (finding M3); unreadable chain two levels deep → `chainUnread` names `iptables -S fw-deep`; a chain narrowed mid-walk (`-A fw-open -i eth0 -j fw-deep`) → not followed.

### Spec and task alignment

- Every `tasks.md` implementation box is `[x]` and maps to diff content: `ip6Walk → iptWalk(tool, skip, exposed)`, the `iptJumps`/`iptJump`/`iptNarrowed`/`iptVerdicts`/`iptJumpMatch`/`iptJumpDropped`/`iptJumpScopeFlags` renames, the v4 walk from `iptablesRead` via `iptChainsRead`, `chainUnread` on `firewallRead` with `ok → unverified` (`withChains`) and `linuxFirewallOK` (`partlyUnread`), the e2e case, the parity-doc and runbook rows, and #763 filed as the declared v6 follow-up.
- The five acceptance criteria are addressable and traced: AC1 (chain allow / port-less allow / two-deep through a loop), AC2 (scoped allow, narrowed jump, `ts-input`, ufw's chains), AC3 (unreadable chain, raw and ufw, missing chain), AC5 (nested, loop, any-address jump), AC4 (e2e). The round-1 Minor about a duplicate `[AC2]` is applied: AC5 exists in `proposal.md` and is cited in `tasks.md`/`verification.md`.
- `iptablesDecide` is untouched and the row `iptables,_a_tailnet_accept_in_an_operator_chain_is_no_rule_for_the_port:_changed_(#762)` pins that, exactly as the proposal's "Decision" paragraph states.
- The v6 read is behaviourally unchanged: `readIP6` passes the same `ufw6-` prefix predicate through `chainWalk.skip`, in the same order relative to `seen`, and `go test -race ./...` is green including `TestIP6Jumps*`/`TestIP6Exposure`. `ip6Exposed` keeps its name (only v6 uses it), as `verification.md` says.
- Scope matches the proposal exactly: one read path, one helper extraction, two doc rows, one e2e case, no dependency, no public contract, no `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tag anywhere in the spec folder.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | spec artefacts (`verification.md`) | The AC2 evidence says removing the `ts-input` skip "turns 83 tests red"; the measured count is **85** failing test lines (round 1 measured 85 as well). The mechanism the sentence asserts is correct, the number cited as evidence is not. | Mutation on the throwaway copy: `sed` out `chain == tsInputChain` in `iptChainsRead`'s skip → `go test ./internal/host/` → 85 `--- FAIL` lines; same probe for the `ufw-` skip → 22, which does match `verification.md`. | The mutation probe itself is the check; no product test covers a documentation number. | spec (`verification.md` only — outside the contract set, safe to correct before archive) |
| Minor | THEORETICAL | resilience / maintainability | `withChains` rebuilds an `ok` step with `newStep(step.Name, OutcomeUnverified, step.Message, fix)` and so **discards `step.Fix`**, while its neighbour `withIP6` prepends it (`if step.Fix != "" { fix = step.Fix + "; " + fix }`). This is latent only because every `OutcomeOK` firewall step today is built by `firewallReadStep`/`firewallSetupFrom` with an empty fix; the first `ok` step that acquires one loses it silently — the same "one field dropped at a seam" class as round 1's Blocker, one layer up. | Code read, `internal/host/linux_steps.go`: `withChains` vs `withIP6` (the latter `step.Fix`-aware, the former not); `firewallReadStep` (`internal/host/step.go:172`) constructs the `ok` step with `""`. No shape in the suite reaches it. | UNTESTED — no row exercises an `ok` step that carries a fix together with `chainUnread`. | code + tests (a row that composes a fix-carrying `ok` step with `chainUnread`), or a comment recording that `ok` steps never carry a fix |
| Minor | THEORETICAL | correctness (residual asymmetry) | The any-address reading is one-sided. Round 1 taught the **jump** predicate (`iptNarrowed`) that `-s`/`-d 0.0.0.0/0` narrow nothing, but the **exposure** predicate the chain walk scores with (`iptScopedAllow`) still reads those literals as a scope, so `-A fw-open -s 0.0.0.0/0 -p tcp -m tcp --dport 3389 -j ACCEPT` is walked into and then reported as clear: a fully open rule silently passes. `proposal.md` declares this predicate and files #743 to change it "for both", so this is spec-consistent — the ask is that #743 be confirmed to carry the exposure side, and that the fix in `iptNarrowed` be judged as not outrunning it. | Scratch probe on the throwaway copy: `-P INPUT DROP` + `-A INPUT -j fw-open`, chain body `-A fw-open -s 0.0.0.0/0 -p tcp -m tcp --dport 3389 -j ACCEPT` → `foreign=[]`, `chainUnread=""`, i.e. check reads `ok`. Control in the same probe: the same allow without `-s` → `foreign=[-A fw-open -p tcp … -j ACCEPT]`. | UNTESTED for the `0.0.0.0/0`-on-the-allow shape (the INPUT analogue is also untested, by #743's scope). | code + tests, inside #743 |
| Minor | REAL | test coverage | The any-address fix from round 1 is pinned only on the `-s` side: `iptables,_a_jump_from_any_address_is_a_jump_for_every_packet:_manual_(#762_review)` uses `-s 0.0.0.0/0`. The `-d` spelling shares the code path (`iptNarrowed` loops `-i`/`-s`/`-d`) and I confirmed it works, but no named row holds it, so a future edit that special-cased only `-s` would pass the suite. | Probe: `-A INPUT -d 0.0.0.0/0 -j fw-open` → `output iptables -S fw-open` recorded, allow reported (`foreign=[-A fw-open -p tcp -m tcp --dport 3389 -j ACCEPT]`); `grep -rn "0.0.0.0/0" internal/host/*_test.go` shows the single `-s` row. | `TestCheckForLinux/iptables,_a_jump_from_any_address_is_a_jump_for_every_packet:_manual_(#762_review)` covers the `-s` spelling only. | tests (one table case, cheap) |
| Minor | REAL | spec artefacts (`tasks.md`, contract set) | The Closing box "features.json replays; verification.md filled; spec review committed" is still `[ ]` although all three now hold: I replayed both `features.json` verification commands (both pass), `verification.md` is filled, and a review exists. A reader cannot tell whether the unticked box or the evidence is authoritative. **Do not tick it before archiving** — `tasks.md` is in the contract set, so editing it invalidates this verdict; record the replay under `verification.md`'s "Review dispositions" instead, or tick it together with the next contract change and re-review. | `tasks.md` "### Closing" vs `features.json` f1/f2 (replayed ok, this session) and this file. | Both `features.json` verification commands, replayed: pass. | spec (`verification.md` now; `tasks.md` only with a re-review) |
| Minor | SPECULATIVE | performance | The walk issues one `iptables -S` per chain reached, sequentially and with no cap, and `chainUnread` joins one note per unreadable chain into an uncapped message. `proposal.md` names the cost (kube-proxy's `KUBE-FIREWALL`) and the walk reads each chain once, so this is a known, bounded-by-the-host trade, not a defect; surfaced only because nothing tests a host with many jumped chains. | Code read of `iptWalk` (queue driven by `r.Output`, `notes` joined with `"; "`); proposal "Risks / open questions", first bullet. | UNTESTED (no row with a large chain set). | — (surface only; do not gate) |

Checked and dismissed, with the evidence:

- **Round 1's Blocker (ufw fold dropped `chainUnread`)** is fixed and its regression test bites: `read.chainUnread = ipt.chainUnread` in `withUFWIptables`, and removing it turns the two ufw-host rows red. AC3's "on a ufw host alike" now holds.
- **Round 1's Major (`0.0.0.0/0` jump not followed)** is fixed on both `-s` and `-d`; the mutation turns its row red (finding M3 records where the *allow*-side reading still differs, by declared scope).
- **AC1 "setup deletes neither"**: `legacySetupStep` deletes only `legacyDeletes(read)` (ufw legacy + `read.copies`), never `foreign`; `TestSetupForLinux/iptables,_an_unscoped_allow_in_an_operator_chain:_scoped_rule_added,_nothing_deleted,_manual_(#762)` records no `-D`.
- **AC1's predicate equals INPUT's**: `iptChainsRead` calls `iptUnscopedIn`, the same body `iptUnscoped` had, and drops the `-A INPUT ` lines the walk returns for INPUT, so the chain lines are added to `foreign` exactly once (no double report).
- **AC2's "not reported" list**: scoped allow, narrowed jump, `ts-input` (85 red without the skip), and ufw's chains on a ufw host (22 red) — `ufw` reaches `iptChainsRead` only through `readUFW → readIptablesOn(…, true)`, and the plain `readIptables` entry keeps `ufw=false`.
- **AC5 loop/nesting**: `seen` is seeded with `INPUT` and set before the read; the loop row and the mid-walk narrowed-chain probe (`-A fw-open -i eth0 -j fw-deep` → not followed) both behave.
- **Port-less chain allow and the SSH caution**: `iptables,_a_port-less_accept_in_an_operator_chain:_manual,_SSH_caution_(#762)` asserts `-A fw-open -j ACCEPT` in the message and "keep loopback, replies and SSH open first" in the fix.
- **Security**: the new path only calls `Runner.Output` with argument slices (no shell, no string-built command), adds no credential or file write, and introduces no dependency. No injection, auth, secret or PII surface in the diff.
- **Missing chain vs unreadable chain**: `chainMissing(err, iptNoChain)` compares the tool's stderr, so exit 1 with `No chain/target/match by that name.` stays silent (`iptables,_a_jump_to_a_missing_chain:_ok_(#762)`) while exit 4 becomes `unverified`.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1/AC2/AC3/AC5 hold and are mutation-proven; AC4 is a VM run I cannot execute, and the any-address reading is asymmetric between the jump and the exposure predicate (declared, #743). |
| Verification       | B | Every criterion names a row and the commands reproduce here (including both `features.json` commands), but one cited count is wrong (83 vs 85) and AC4's 135-pass figure is not reproducible in a review run. |
| Scope              | A | Diff matches the proposal exactly: one read path, one helper extraction, two doc rows, one e2e case, no dependency, no public contract. |
| Reliability        | B | Error paths are handled — the round-1 dropped-unknown is fixed and pinned — with one latent field-drop left at `withChains` (`step.Fix`). |
| Maintainability    | B | Small functions, comments explain why, names are now consistent (`ipt*` shared, `ip6Exposed` kept); `iptablesRead` still carries its pre-existing gocyclo waiver. |
| Handoff-readiness  | B | Parity row and two runbook rows added, #763 filed, `verification.md` names a test per criterion; the `tasks.md` Closing box is stale against the evidence. |

### Verdict

**PASS** — no Blocker, no REAL Major; every open finding is a Minor (two REAL, three THEORETICAL/SPECULATIVE), and the rubric has no C or D. `dotf spec archive` is **advisable** in this state: the verdict is fresh against the three contract digests, and nothing in the contract set needs to change for it.

### Recommended next steps

All of these are dispositionable in `verification.md` (outside the contract set) — none requires a contract edit, and doing one now would invalidate this verdict.

1. **spec (`verification.md`)** — correct "83" to the measured 85 for the `ts-input` skip mutation (M1). Two-word edit, no code.
2. **code + tests** — preserve `step.Fix` in `withChains` as `withIP6` does, or record in a comment why `ok` steps never carry one (M2). Not blocking: no shape reaches it today.
3. **#743 (code + tests)** — confirm the exposure predicate (`iptScopedAllow`) learns that `-s`/`-d 0.0.0.0/0` narrow nothing, now that `iptNarrowed` already does on the jump side (M3); the spec declares this predicate as #743's, so this is a hand-off, not a defect in this change.
4. **tests** — add the `-d 0.0.0.0/0` spelling beside the `-s` row so the fix is pinned on both sides (M4).
5. **spec (`tasks.md`, contract set)** — leave the Closing box as it is for this archive; record the `features.json` replay and the review in `verification.md` and tick the box only alongside the next contract change (M5).
6. **follow-up (#763)** — the v6 read still reports `ok` on a chain it cannot read; that convergence is out of this spec's scope and already filed.

UNVERIFIED in this review, stated rather than assumed:

- **AC4 — `make e2e-linux`** (`verification.md`: 135 pass, 0 fail on `51d753f`; `features.json` f3). Driving `dotf vm` Linux guests is out of reach for a review run. The e2e case reads correctly against AC1/AC4 (a tailnet accept is `ok`; an unscoped allow is `manual` naming `-A tsbe2e4 -p tcp -m tcp --dport 3389 -j ACCEPT` with `firewall_ok` false; setup leaves it; `ok` again after removal), and the guest-cleanup fallback (`iptables -N tsbe2e4 || iptables -F tsbe2e4`) makes it re-runnable after an aborted run.
- **`gosec ./...`** — not in this environment; the task claims no local run, and the CI `security` job is the owner. Nothing in the diff adds a `#nosec`-worthy path (only `Runner.Output` with slice args).
- **PR/CI state** — not inspected; this review judges the diff at `afea4e9`, not the PR's checks.
