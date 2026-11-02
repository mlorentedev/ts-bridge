---
spec: "BUG-492-ts-input-chain"
verdict: "PASS"
reviewed_sha: "83135453203a21ec47851ec8ca78af65830eebad"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-30"
---

## Adversarial review

**Scope**: BUG-492-ts-input-chain — `git diff 2ba86bd01b634de70fa1765582c5bde39373d1ad...HEAD` (4 commits: `cd0a72b`, `51e5eff`, `e828341`, `8313545`), 394 insertions / 20 deletions across `internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `docs/`, and the spec folder. This round reads the whole change — the round-1 fixes are judged against the findings they claim to close, not trusted.
**Sources**: `specs/BUG-492-ts-input-chain/{proposal,tasks,verification}.md`, `features.json`, `review-request.json`, `review-round1.md`; contract digests recomputed with `dotf spec digest BUG-492-ts-input-chain` — identical to `review-request.json` (`features.json` `b8fd1b3a…`, `proposal.md` `eaefadcf…`, `tasks.md` `c8a6a124…`), so this verdict describes the contract on disk at `8313545`. Fixture-vs-upstream check re-derived from `tailscale.com@v1.104.2` in the module cache.

### Spec and task alignment

- AC1–AC5 each map to a named test that exists and passes, and round 2 removed the one mapping that was helper-only: AC2's wiring is now pinned through `readIptables` by `TestIptablesRule/a deny after the jump is shadowed by ts-input` and `TestSetupForLinux/iptables, a deny after the jump is shadowed by ts-input`.
- `tasks.md`: every Implementation and Closing box is ticked except "PR opened referencing this spec folder", correct for the verification window. No `[x]` is unsupported by the diff.
- `proposal.md` carries its acceptance criteria as unticked boxes. That is this repo's convention for the verification window, not drift — `specs/archive/BUG-035`, `BUG-483` and `BUG-481` all archive with `- [ ]` ACs (grepped: 0 ticked in each). No finding.
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags remain in any spec file.
- Code shape matches the proposal exactly: `readLinuxFirewall` keeps the elevated/ufw/no-backend ladder and delegates to `readIptables`; `iptablesDecide` and the BUG-035/483/481 parser are untouched, so "the existing decision rules apply unchanged to every line" is a property of the code, not a claim about it. `iptInlineChain` replaces only a line whose fields equal `-A INPUT -j ts-input`, and `iptTSInputJump` is exact-field equality, so a qualified jump (`-i eth0`) stays opaque as AC3 requires.
- The declared behaviour on tailscaled's own chain is what the parser actually does, and I traced each line: the loopback ACCEPT is dropped by `iptLoopback`; `-i tailscale0 -j ACCEPT` is port-less-covered → `rulePresent`; the CGNAT `RETURN`/`DROP` carry `-s` + `!` and fail `iptPortlessToken` → non-deciding; the magicsock ACCEPT is `-p udp` → non-deciding. `-N ts-input` is skipped by the `f[0] != "-A"` guard, and the fixture exercises it.
- Round-1 dispositions, each re-checked rather than inherited: **F1 applied** (call-site coverage added), **F2 applied** (false mutation claim replaced with measured ones), **F3 applied** (red count replaced with the figure I reproduced), **F4 applied** (`assertFirewallMessage` + reason assertion), **F5 declined with a written rationale** in `verification.md`, **F6 ticketed** — `gh issue view 530` returns an OPEN issue in this repo with root cause, two options and acceptance criteria. Debt is not floating.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | SPECULATIVE | AC5 fixture fidelity | `tsInputDefault`'s intra-chain order is the replay's order, not a live host's. Upstream `router_linux.go` appends `-i tailscale0 -j ACCEPT` (via `AddBase`, :844), then `AddMagicsockPortRule` (:848/853), then `AddExternalCGNATRules` (:893) — so a real `iptables -S ts-input` puts the UDP 41641 ACCEPT **before** the ChromeOS `RETURN` and CGNAT `DROP`, while the fixture puts it last. No answer changes: the two deciding rules (loopback, skipped; `tailscale0` ACCEPT, present) keep their position in every order, and every rule that moves is non-deciding by construction. | Upstream read; probe with the live order as `iptChain` through `readLinuxFirewall` → `rulePresent` (state 2), same as the fixture. | `TestIptablesTSInput/tailscaled default` and `… tailscaled's other rules decide nothing` pin the outcome; the ordering claim itself is UNTESTED | spec artifact (`verification.md`: record that the order is the replay's, not the router's — `proposal.md`'s risk bullet already says the replay pins the printed form, not runtime behaviour, so nothing stronger is owed) |
| Minor | REAL | reliability (accepted risk, carried from round 1 F5) | The two reads are not atomic. tailscaled's restart is `AddChains` → `ClearChain("filter","ts-input")` then repopulate; a read that takes INPUT before the clear and `ts-input` during it sees the jump with an empty chain → absent → setup appends the unscoped ACCEPT. The window fails toward the pre-#492 behaviour, never toward a false "present". | Upstream `AddChains`/`ClearChain` read; `verification.md` records the disposition ("declined as an accepted risk", with why closing it needs an atomic `iptables-save` read the proposal does not cover). Not reproduced. | UNTESTED (deliberately: a window no table test pins; the fail-direction is benign) | code or ticket — no action needed for this change; the disposition is written |
| Minor | SPECULATIVE | quality (code hygiene) | `tsInputJump` is a mutable package-level `var []string` used only as a comparison operand, and the jump match is computed twice (once by `slices.ContainsFunc` in `readIptables`, once per line in `iptInlineChain`). No defect today: both call sites share `iptTSInputJump`, so they cannot disagree. | Code read, `internal/host/linux_steps.go:181-222`; probe with a doubled jump line read the chain once and returned the correct state. | `TestIptablesTSInput/*` pins the behaviour; the shape is UNTESTED and needs no test | code (optional: an array or a `tsInputJump()` constructor), or leave as is |
| Minor | REAL | AC3 coverage asymmetry | AC3's "no second read runs" is pinned on the setup path (`TestSetupForLinux/iptables, no ts-input jump: one read`, `… a qualified ts-input jump is not followed`) but `TestCheckForLinux` has no row without a jump in its fixture, so the negative case is asserted on one of the two commands AC3 covers. Exposure is low: there is one read function and the check step only formats its result. | `linux_steps_test.go` `TestCheckForLinux` table — every iptables row's fixture contains `-A INPUT -j ts-input`. | `TestSetupForLinux/iptables, no ts-input jump: one read` (setup only); the check-path row is UNTESTED | tests (optional: one row with `-P INPUT ACCEPT\n` and a call list omitting `output iptables -S ts-input`) |

Everything else I attacked and could not break:

- **Splice position.** Mutation `rules = rules + "\n" + string(chain)` — the round-1 survivor — now fails `TestIptablesRule/a deny after the jump is shadowed by ts-input` and `TestSetupForLinux/iptables, a deny after the jump is shadowed by ts-input`. Measured: exit 1, 4 `--- FAIL` lines. F1 is genuinely closed.
- **Ordering in front.** Mutation prepending the chain to the whole INPUT text fails `TestIptablesTSInput/deny before the jump decides first`, `… a qualified jump is not followed` and `… no jump, nothing to follow`. A probe through `readLinuxFirewall` with a deny before the jump returns `ruleBlocked` naming that INPUT rule, so AC2's "before" direction holds at the call site too, not only in the helper.
- **Jump matching.** Mutation `strings.HasSuffix(strings.TrimSpace(line), "-j ts-input")` inside `iptTSInputJump` fails `TestSetupForLinux/iptables, a qualified ts-input jump is not followed` and `TestIptablesTSInput/a qualified jump is not followed` — both rows `verification.md` claims.
- **Read error.** Mutation `_ = chain` fails 5 subtests; ignoring the error, and dropping the command from the reason (`iptables: %v`), each fail the three `ts-input unreadable` rows. `assertFirewallMessage` is not vacuous: it is the mutation that turns those rows red.
- **Red count.** Forcing the jump check false (pre-fix behaviour) gives exactly **19 subtest FAILs and 22 `--- FAIL` lines**, the figure `verification.md` now records. F3 is closed.
- **Build/verification:** `go build ./...` exit 0; `go test -race -count=1 ./...` all packages ok; `gofmt -l internal/host` empty, repo-wide `gofmt -l .` = 8 files none in `internal/host` (pre-existing, as claimed); `GOOS=linux|windows|darwin go vet ./...` clean; `golangci-lint run ./internal/host/...` → 0 issues; `scripts/check-lessons.sh` → `OK (49 lessons)`.
- **Fixture arguments vs upstream:** `buildMagicsockPortRule` is `{"-p","udp","--dport",port,"-j","ACCEPT"}` — the fixture's extra `-m udp` is what iptables prints for an implied match, so it is the printed form, not an invented flag. `buildExternalCGNATRules(CGNATModeDrop)` is RETURN(ChromeOS 100.115.92.0/23) then DROP(CGNAT 100.64.0.0/10) on `! -i tun`, matching the fixture's rule order and its `-s … ! -i …` print order. `AddLoopbackRule` inserts `-i lo -s <addr> -j ACCEPT` at position 1, printed source-first as the fixture has it.
- **`features.json`:** all four `verification` commands run as written and exit 0 (f1/f4 including their `grep -q` on the subtest name). Every entry is `state: pending` with empty `evidence`, the only state an agent may write.
- **Scope:** no production caller other than `firewallSetupStep`/`checkLinux` reads the firewall (`iptablesCovers`/`ufwAllows` are test-only helpers), so no second read path was left inconsistent with the new behaviour. `cmd/cli/host*.go` only render the step results; `internal/proxy` mentions the firewall in an error hint. #530 (ufw) is the separate, ticketed residual and is outside this proposal by its own "Out of scope" section.
- **Docs:** `lesson-050` states the general rule, its limit, and how the fixture was obtained, including that the replay pins the printed form, not runtime behaviour; `title: "2026-10-30"` matches lessons 044/046/047/048, so it is the convention, not an omission. The parity row names the new behaviour and the #526 boundary.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | AC1–AC5 verified here, negative paths (unreadable chain, deny before/after, qualified jump, no jump) all pinned; no observed defect. |
| Verification       | A | Every recorded claim reproduced this session: five mutation results, the 19/22 red count, race suite, vet ×3, lint, gofmt, `check-lessons`, the four `features.json` commands, the contract digests, and the fixture argument sets against upstream. |
| Scope              | A | Diff matches the proposal; the only extras are the lesson, its index row and the parity row, all named by `tasks.md`. |
| Reliability        | B | Fails closed on a failed read, never appends on unknown; the un-atomic two-read window is real and documented with a reasoned disposition rather than left silent. |
| Maintainability    | A | `readIptables` ~14 lines, `iptInlineChain` ~8, `iptTSInputJump` 1, cyclomatic complexity well under 10, no dead code, comments say why the jump's position decides. |
| Handoff-readiness  | A | Lesson + index + parity row + corrected `verification.md` with per-finding dispositions + `features.json` + #530 filed; only the PR box is open, which is the verification window. |

### Verdict
PASS

No Blocker, no REAL Major. The four residual findings are Minor — one a written accepted risk, one a coverage asymmetry on a shared code path, one an evidence-precision note whose impact is nil, one pure hygiene — and the rubric is all A/B with no C or D. Round 1's FAIL rested on two REAL Majors; both are closed, and I re-proved the closures by mutation rather than by reading the author's new claims.

### Recommended next steps

Everything below is **outside the contract set**. `proposal.md`, `tasks.md` and `features.json` are unchanged and must stay that way — the digests in `review-request.json` match the files this verdict describes, and an edit there would invalidate it.

1. **No action, record only (F5/TOCTOU).** The accepted-risk disposition already in `verification.md` is sufficient; if the owner wants a durable home, fold it into #526 rather than opening a new ticket.
2. **`verification.md`, optional (AC5 ordering note).** Say in one line that `tsInputDefault`'s rule order is the replay's, not `router_linux.go`'s (magicsock is appended before the CGNAT rules upstream), and that no deciding rule's position changes. That keeps the next reader from re-deriving it.
3. **Optional, tests (AC3 coverage asymmetry).** Add one `TestCheckForLinux` row whose INPUT holds no jump and whose expected call list omits `output iptables -S ts-input`. Three lines; ticket it if it is not worth this PR's diff.
4. **Optional, code (hygiene).** Make `tsInputJump` an array (`[...]string`) or a function, so the comparison operand is not package-level mutable state. Cosmetic; the current shape is correct.
5. **Board.** #492 is the ticket this change closes, and #530 is filed and OPEN. Closing #492 with the merging PR — saying there that #530 and #526 carry the rest — is what makes the board match reality.
6. **Then archive.** `dotf spec archive BUG-492-ts-input-chain` is **advisable** in this state: the review is fresh, `reviewed_sha` is HEAD, and the contract digests match.

Gate internals I did not exercise (implemented outside this repository, out of scope for this run): the `review.md` frontmatter parse, the reviewer-pool string match on `nan/deepseek-v4-flash`, and the staleness check. UNVERIFIED by me.

### Notes on what I could not verify

- The `ubuntu:24.04` end-to-end run in `verification.md` (real iptables, `NET_ADMIN`, replayed tailscaled rules) was not re-run: no container runtime is reachable from this session. UNVERIFIED here; the fixture's rule arguments and printed-form conventions are what I could check, and they match `tailscale.com@v1.104.2`'s builders.
- Whether a live host's `iptables -S ts-input` matches the fixture line-for-line, position included, is exactly the gap the proposal's risk bullet discloses; my finding above narrows it to the non-deciding rules.
