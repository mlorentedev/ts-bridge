---
spec: "BUG-492-ts-input-chain"
verdict: "FAIL"
reviewed_sha: "e82834137ef86ec73176a1e6b7bcc557f2d863e8"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-30"
---

## Adversarial review

**Scope**: BUG-492-ts-input-chain — `git diff 2ba86bd01b634de70fa1765582c5bde39373d1ad...HEAD` (3 commits: `cd0a72b`, `51e5eff`, `e828341`), 350 insertions across `internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `docs/`, and the spec folder.
**Sources**: `specs/BUG-492-ts-input-chain/{proposal,tasks,verification}.md`, `features.json`, `review-request.json`; contract digests re-checked with `dotf spec digest BUG-492-ts-input-chain` (identical to `review-request.json`).

### Spec and task alignment

- AC1–AC5 are each named to at least one test in `verification.md`, and for AC1, AC2 (unit level), AC3 and AC4's *outcome* the named tests exist and pass. AC2's **wiring** (the chain is spliced at the jump inside `readIptables`, not merely by a helper) has no covering test — see finding F1.
- `tasks.md`: every Implementation and Closing box is ticked except "PR opened referencing this spec folder", which is correct for the verification window. No `[x]` is unsupported by the diff.
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags remain in the spec files.
- The production diff is small and honest: `readLinuxFirewall` keeps its ufw/no-backend logic and delegates the iptables branch to a new `readIptables`; `iptablesDecide` and the BUG-035/483/481 parser are untouched, so the decision rules really do apply unchanged to the inlined lines, as the proposal claims.
- `iptInlineChain` splices the raw `iptables -S ts-input` text (including tailscaled's `-N ts-input` line) into INPUT in place of the jump. I confirmed the `-N` line is inert: `iptablesDecide` skips any line whose first field is not `-A`, and `TestIptablesTSInput/tailscaled default` exercises a fixture that contains `-N ts-input`.
- I checked the fixture (`tsInputDefault`) against upstream, not against the author's description: `tailscale.com@v1.104.2/util/linuxfw/iptables_runner.go` builds the hook with `Insert("filter","INPUT",1,"-j","ts-input")` → `-A INPUT -j ts-input`; `AddLoopbackRule` with `Insert("filter","ts-input",1,"-i","lo","-s",addr,"-j","ACCEPT")`; `addBase4` with `-i <tun> -j ACCEPT`; `buildExternalCGNATRules(CGNATModeDrop)` with `{"!","-i",tun,"-s",ChromeOSVMRange,"-j","RETURN"}` and `{"!","-i",tun,"-s",CGNATRange,"-j","DROP"}`; `AddMagicsockPortRule` UDP. The fixture's lines match those argument sets in `iptables -S` canonical print order (`-s` before `-i`). AC5 holds as far as it can be checked without a live tailscaled, and the proposal is explicit that the replay pins the printed form, not runtime behaviour.
- `readIptables` fails closed on the second read: a failed `iptables -S ts-input` returns `ruleUnknown` with `reason = "iptables -S ts-input: <err>"`, which `firewallSetupStep` renders as `failed` (no append) and `firewallCheckStep` as `unverified`. AC4's *behaviour* is implemented; its *message* is untested (F3).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | tests / AC2 wiring | Nothing pins that the chain is spliced **at the jump** inside `readIptables`. Replacing `rules = iptInlineChain(rules, string(chain))` with `rules = rules + "\n" + string(chain)` (append the chain at the end) leaves the whole suite green. The only position test calls `iptInlineChain` directly and never goes through `readIptables`, so the helper is tested while its call site is not. | Mutation run in this session: with the append-at-end edit, `go test ./internal/host/ -count=1` → exit 0, 0 failures. A probe test I added and then removed, calling `readIptables` on `-P INPUT ACCEPT\n-A INPUT -j ts-input\n-A INPUT -p tcp -m tcp --dport 3389 -j DROP` + `tsInputDefault`, returns `rulePresent` with the real code and `ruleBlocked` under the mutation — i.e. exactly the AC2 property "a deny after the jump is shadowed" is unprotected. | UNTESTED (`TestIptablesTSInput/deny after the jump is shadowed` covers the helper only; no `TestSetupForLinux`/`TestIptablesRule` row has a deny after the jump) | tests — add a `TestIptablesRule` (or `TestSetupForLinux`) row whose INPUT holds a deny after the jump and whose chain holds the tailnet ACCEPT, asserting `rulePresent`/`ok`; it must fail under the append-at-end mutation |
| Major | REAL | spec evidence (verification.md) | `verification.md` records a mutation check that does not reproduce: "appending the chain at the end instead of at the jump fails `deny after the jump is shadowed`". It does not fail — see F1. The artifact the archive gate trusts therefore asserts a coverage claim that is false. | Same run as F1: exit 0, 0 failures, and `--- PASS: TestIptablesTSInput/deny_after_the_jump_is_shadowed`. | — (evidence, not a test) | spec artifact (`verification.md` is outside the contract set, so correcting it does not invalidate this review) |
| Minor | REAL | spec evidence (verification.md) | Red-first count does not reproduce: "with `iptInlineChain` stubbed to return INPUT unchanged, 19 cases fail". Stubbing it to `return input` fails 5 subtests / 9 `--- FAIL` lines (`TestIptablesRule/present_inside_ts-input`, `TestSetupForLinux/iptables,_tailscaled's_ts-input_accepts_the_tailnet`, `TestCheckForLinux/…`, `TestIptablesTSInput/tailscaled_default`, `TestIptablesTSInput/deny_after_the_jump_is_shadowed`). | Measured in this session with `-v`. | — | spec artifact (`verification.md`) |
| Minor | REAL | AC4 test traceability | AC4 says "Both name the command", but no test asserts the reason text. `TestSetupForLinux` compares `Name`+`Outcome` only; `TestCheckForLinux` compares `Name=Outcome`; only blocked steps get a message assertion (`assertNamesBlockedRule`). A regression dropping `iptables -S ts-input` from the reason would pass. | Code read of the two test harnesses (lines ~390–408, ~578) and `linux_steps.go` `firewallSetupStep`/`firewallCheckStep`. | UNTESTED | tests — assert the message/reason contains `iptables -S ts-input` on the unreadable rows |
| Minor | THEORETICAL | reliability (TOCTOU) | The two reads are not atomic. tailscaled's restart path is `AddChains` → `ClearChain("filter","ts-input")` then repopulate; a `host setup` that reads INPUT *before* the clear and `ts-input` *during* it sees the jump with an emptied chain → `absent` → appends `-A INPUT -p tcp --dport 3389 -j ACCEPT`, i.e. re-opens the port on every interface, which is the defect #492 exists to remove. Narrow window, and it fails toward the old bug, never toward a false "present". | Upstream read (`AddChains`/`ClearChain` in `iptables_runner.go`); no reproduction. | UNTESTED | code (optional: re-read both after a mismatch) or ticket |
| Minor | REAL | scope / residual defect | Declared out of scope: `readLinuxFirewall` prefers `ufw`, whose `show added` never contains tailscaled's rules, so on a stock Ubuntu host (ufw installed) the #492 path is unchanged and `host setup` still runs `ufw allow 3389/tcp`, which is not interface-scoped. The proposal explains the omission but names no follow-up ticket (follow-up #526 covers IPv6, `nodivert` and nftables only). | Code read of `readLinuxFirewall` (ufw branch first) and the proposal's "Out of scope" ufw bullet. | UNTESTED | ticket (Standing Order #4: no floating debt) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1–AC5 behaviourally met and verified by probe; the negative-path wiring (F1) and one message guarantee (F3) are unguarded. |
| Verification       | C | Most recorded evidence reproduces (race suite, `golangci-lint` 0 issues, `gofmt -l` = the 8 pre-existing files, `check-lessons` OK, fixtures vs upstream), but one mutation claim is false (F2) and one count is wrong (F3). |
| Scope              | A | Diff matches the proposal exactly; no creep; docs (`lesson-050`, parity row) fit the decision. |
| Reliability        | B | Fails closed on read error, no append on unknown, no chain-loop recursion; the two-read TOCTOU window is unaddressed but low-exposure. |
| Maintainability    | A | `readIptables` ~14 lines, `iptInlineChain` ~8, `iptTSInputJump` 1, CC well under 10, no dead code, comments explain why the jump's position matters. |
| Handoff-readiness  | B | Lesson, index row, parity row, `verification.md` and `features.json` present; `verification.md` carries two inaccurate claims and the PR is not yet opened. |

### Verdict
FAIL

Two **REAL Major** findings (F1, F2): a surviving mutation on AC2's headline semantic — "the chain's rules decide at the jump's position" — plus a verification artifact that claims that mutation was caught. The rubric alone (one C, no D) would permit PASS WITH GAPS; the severity × reality axis escalates it.

### Recommended next steps

Both required fixes are **outside the contract set**, so none of `proposal.md`, `tasks.md`, `features.json` needs to change and the digests in `review-request.json` stay valid.

1. **Tests (flips F1).** Add one row to `TestIptablesRule` or `TestSetupForLinux`: INPUT `-P INPUT ACCEPT\n-A INPUT -j ts-input\n-A INPUT -p tcp -m tcp --dport 3389 -j DROP\n` with `iptables -S ts-input` = `tsInputDefault`, asserting `rulePresent` / `OutcomeOK`. Red-green it: it must fail with `rules = rules + "\n" + string(chain)` in `readIptables` and pass with `iptInlineChain`. That is the mutation `verification.md` already claims to have run.
2. **`verification.md` (flips F2, and F3).** Replace the false mutation result with the measured one, and correct the red-first count (or say how it was counted).
3. **Tests (F3).** Assert that the setup/check message for the unreadable chain contains `iptables -S ts-input`, so AC4's "both name the command" is traceable.
4. **Ticket (F6).** File the ufw follow-up: on hosts where ufw is installed, `host setup` still opens the port on every interface. No code change needed for this review to pass; it is Standing Order #4 debt, not a scope violation.
5. **Optional (F5).** Either note the two-read TOCTOU in `verification.md` as an accepted, ticket-tracked risk, or close it by re-reading both chains when the inlined result differs from the INPUT-only result.
6. Then **re-review** (this session implemented nothing here, so a fresh pool draw is not required for independence, but the gate wants a fresh passing `review.md`): the verdict flips to PASS on steps 1–3 alone.

`dotf spec archive` is **not advisable** in the current state: the spec's own evidence contains a claim I could not reproduce, and the mutation it claims to catch passes the suite.

### Notes on what I could not verify

- The end-to-end `ubuntu:24.04` run recorded in `verification.md` (tailscaled rules replayed with `NET_ADMIN`, `host setup`/`host check` against real iptables) was not re-run: no container runtime is reachable from this session. UNVERIFIED, but consistent with the fixture-vs-upstream check above.
- The `iptables -S ts-input` printed form itself is not reproducible here (needs iptables + root); I verified the rule *arguments* against the upstream builders, which is what the printed form is derived from.
