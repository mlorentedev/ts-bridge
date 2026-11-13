---
spec: "BUG-762-v4-operator-chains"
verdict: "FAIL"
reviewed_sha: "a4e0f41e617cca599702dbc6eb3c446a818c6dd6"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: `BUG-762-v4-operator-chains` — whole change, `git diff f8ace2d22e7b22b89ba6261cb04a57563a4fea2e...HEAD` (5 commits: `9414418` fix, `6819422` docs, `0b8ca47`/`c4d8077`/`a4e0f41` spec), reviewed at `a4e0f41`.
**Sources**: `specs/BUG-762-v4-operator-chains/{proposal,tasks,verification,features.json}`; `internal/host/linux_steps.go`; `internal/host/linux_steps_test.go`; `scripts/e2e/linux.sh`; `docs/design/host-client-parity.md`; `docs/runbooks/guide-rdp-host-setup.md`.

Evidence produced in this session (fresh, in this worktree):

- `go build ./...` → exit 0.
- `go test -race ./...` → all packages `ok`, 0 failures (20.9s).
- `golangci-lint run` (2.14.0, the mise pin) → `0 issues.`; `gofmt -l .` empty; `go vet ./...` clean.
- `dotf spec digest BUG-762-v4-operator-chains` → all four digests identical to `review-request.json` (`proposal.md 1c09fdfd…`, `tasks.md 1e8c3d32…`, `features.json 928e5c07…`), so this verdict measures the contract as launched.
- Mutation probes (all reverted; `git status` clean apart from this file and the launcher's `review-request.json`):
  - drop `chainUnread` from `linuxFirewallOK` → `TestCheckForLinux/iptables,_an_operator_chain_that_cannot_be_read:_unverified_(#762)` red.
  - drop the `ts-input` skip → 85 failing test lines; drop the `ufw-` skip → 22 failing test lines (verification.md claims 22 exactly — reproduced).
  - never enqueue a reached chain (one-level walk) → the four `#762` check/setup rows and `TestIP6Jumps/two_jumps_deep` red.
- Two-purpose scratch tests written under `internal/host/`, run, and deleted (reproductions quoted in the findings below).

### Spec and task alignment

- `tasks.md` implementation boxes are all `[x]` and each maps to diff content: `iptWalk(tool, skip, exposed)` + `iptJumps`/`iptJump` rename, v4 walk from `iptablesRead`, `chainUnread` on `firewallRead` with `ok → unverified`, e2e case, parity-doc and runbook rows, #763 follow-up named. `verification.md` names the rows per criterion. The **Closing** section is entirely `[ ]` although `verification.md` records `go test -race ./...`, `golangci-lint run` and `make e2e-linux` as run (minor finding M2).
- `iptablesDecide` is genuinely untouched (pinned by the "changed (#762)" row), and the v6 read is behaviourally unchanged: `readIP6` passes the same `ufw6-` prefix predicate through `chainWalk.skip`, in the same order relative to `seen` as before, and `go test -race ./...` is green, including `TestIP6Jumps`, `TestIP6JumpsUFWHost`, `TestIP6Exposure`.
- Scope matches the proposal: one read path, one helper extraction, two docs rows, one e2e case, no dependency, no public contract. No creep found.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Blocker | REAL | correctness / AC3 | On a **ufw host**, a non-ufw operator chain that INPUT jumps to and that cannot be read is silently dropped: the read reports `ok` with `firewall_ok: true`, and `setup` reports `ok`, even though `iptables -S <chain>` failed. `iptChainsRead` sets `chainUnread`, but `withUFWIptables` folds only `copies`, `inputOpen`, `policyUnread`, `foreign`, `scoped`, `copiesAdmit` and `state` — the new field never crosses into the folded read, so `partlyUnread`/`withChains` never see it. AC3 states the rule with no ufw exemption, and the proposal's "unreadable chain reads as unknown, not as clear" is exactly what is violated here. | Reproduced with a scratch test: ufw host (`ufw allow in on tailscale0 to any port 3389 proto tcp`), `iptables -S INPUT` = `ufwHostInput + "-A INPUT -j fw-open"`, `iptables -S fw-open` → exit 4. Recorded calls contain `output iptables -S fw-open`; result was `chainUnread=""`, check `outcome=ok`, message `"Firewall allows port 3389"`, `linuxFirewallOK=true`; `firewallSetupStep` → `outcome=ok`. The readable twin (same host, `-A fw-open -p tcp -m tcp --dport 3389 -j ACCEPT`) *is* reported `manual`/`firewall_ok=false`, which isolates the dropped field. | UNTESTED — every `#762` unreadable-chain row "an operator chain that cannot be read: unverified", "the scoped rule in place and a chain that cannot be read: unverified" runs on a plain iptables host; no ufw-host row exercises `chainUnread`. | code (`withUFWIptables`: carry `ipt.chainUnread`) + tests (a ufw-host check row and a ufw-host setup row) |
| Major | REAL | correctness / AC1 | A v4 jump whose scope is the any-address literal is treated as narrowed, so the chain is never read: `-A INPUT -s 0.0.0.0/0 -j fw-open` (and `-d 0.0.0.0/0`) leaves an unscoped allow inside `fw-open` unreported. The shared predicate is `ip6Narrowed`, which special-cases only the IPv6 any-address (`v != "::/0"`); it was written for v6 and the spec's claim that the v4 walk "follows the same jumps as v6" does not hold for the v4 spelling of "any address". Nothing in `ip6Jump*` is v4-aware, and the proposal's own risk note ("a jump with a match the parser does not model") does not cover this — the scope *is* modelled, it is just modelled with the wrong literal. | Reproduced with a scratch test on an iptables host: `-A INPUT -s 0.0.0.0/0 -j fw-open` with `-A fw-open -p tcp -m tcp --dport 3389 -j ACCEPT` → recorded calls hold **no** `output iptables -S fw-open`, `foreign=[]`, check `outcome=ok`, `firewall_ok=true`. The unnarrowed control (`-A INPUT -j fw-open`) in the same probe yields `foreign=[-A fw-open -p tcp … -j ACCEPT]`, `outcome=manual`, `firewall_ok=false`. | UNTESTED for v4. The v6 analogue is named: `TestIP6Jumps/a_jump_on_every_interface_but_one_is_followed`; no `0.0.0.0/0` row exists anywhere in `internal/host` (`grep` finds `::/0` only). | code (a v4-aware "narrows nothing" predicate for `0.0.0.0/0`, e.g. `ip6Narrowed` learning both literals) + tests (a `TestCheckForLinux` row and a walk-level row) |
| Minor | REAL | maintainability | The walk is now shared, but four of its helpers keep v6 names that the v4 path calls: `ip6Verdicts` (in `iptJump`), `ip6Narrowed`, `ip6Match`, `ip6Dropped`, `ip6JumpCovers`. The task only renamed `ip6Jumps`/`ip6Jump`. This is the seam that hid the Major above: an IPv4 walk governed by a function documented and specialised for `::/0`. | Code read of `iptJump`/`ip6Narrowed`; `grep -n "ip6" internal/host/linux_steps.go` shows the v4 path entering all five. | n/a (naming; no behaviour test applies) | code |
| Minor | REAL | spec artifacts | `tasks.md` **Closing** has all four boxes `[ ]` while `verification.md` records those commands as run with results, and the review (`git log`) shows the verification commit `a4e0f41`. A reader cannot tell whether the box or the evidence is authoritative. | `tasks.md` lines "### Closing" vs `verification.md` "## Commands". | n/a | spec (`tasks.md`) — contract set, so on this FAIL round it belongs with the re-review |
| Minor | REAL | spec artifacts | The proposal's acceptance list carries **two** bullets tagged `[AC2]`; the second ("A chain reached through another chain is reported. A jump loop ends.") has no AC number of its own while `tasks.md` and `verification.md` cite AC1–AC4 only. Traceability from criterion to task to test breaks at exactly the nested/loop requirement. | `proposal.md` "## Acceptance criteria" — `[AC1]`, `[AC2]`, `[AC2]`, `[AC3]`, `[AC4]`. | The behaviour is covered by `TestCheckForLinux/iptables,_an_allow_two_jumps_deep,_through_a_loop:_manual_(#762)`; the *criterion label* is untested/unaddressable. | spec (`proposal.md`) — contract set, so with the re-review |

Not findings (checked and dismissed, with the evidence):

- AC2's "not reported" list holds on the paths tested: a scoped (`-i tailscale0`) allow in a chain (`TestCheckForLinux/iptables,_a_scoped_allow_in_an_operator_chain:_ok_(#762)`), a narrowed jump (`…behind_a_narrowed_jump_is_not_read:_ok_(#762)`, and the recorded call list proves no chain read), `ts-input` (mutation: 85 red) and `ufw-*` (mutation: 22 red, matching the verification claim).
- Setup never deletes a chain rule it did not write: `legacySetupStep` deletes only `legacyDeletes(read)` (ufw legacy + `read.copies`), and `TestSetupForLinux/iptables,_an_unscoped_allow_in_an_operator_chain:…_nothing_deleted,_manual_(#762)` records no `-D`.
- Loop termination and "read once" hold (`seen` seeded with `INPUT`, set before the read) and are pinned by the loop row plus `TestIP6Jumps/a_loop_ends` and `…a_chain_jumped_to_twice_is_read_once`.
- "Missing chain is skipped silently" is pinned by `TestCheckForLinux/iptables,_a_jump_to_a_missing_chain:_ok_(#762)` (stderr `No chain/target/match by that name.`) and the `TestIP6Jumps` rows for non-chain targets; a permission-denied read quotes the tool and yields `unverified` (scratch probe).
- No injection, secret, new dependency or side-effecting call in the new path: the walk only calls `Runner.Output`, arguments are passed as a slice, and `readIptables`' non-ufw entry point keeps `ufw=false`.
- The decision side is unchanged as declared (row "a tailnet accept in an operator chain is no rule for the port: changed (#762)").

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | AC1/AC2 hold on the main paths, but AC3 silently misses ufw hosts and AC1 misses a jump spelled with `0.0.0.0/0`; both reproduced. |
| Verification       | B | Every criterion has a named row and the commands reproduce here, but the AC3 evidence claims coverage that the ufw path does not have, and AC4 (135 pass) is a VM run this review cannot re-execute. |
| Scope              | A | Diff matches the proposal exactly: one read path, two doc rows, one e2e case, no dependency, no public contract. |
| Reliability        | C | Error paths are handled well except the one that matters: the unreadable-chain signal is dropped when the read is folded through ufw, turning an unknown into a pass. |
| Maintainability    | B | Small functions, clear comments, `iptChainsRead` is 6 lines; the v6-named helpers shared with the v4 walk are the one smell, and it is not cosmetic. |
| Handoff-readiness  | B | Docs updated (parity row, runbook rows), #763 filed as the declared follow-up; `tasks.md` Closing stale against `verification.md`. |

### Verdict

FAIL — one **REAL Blocker** (AC3 dropped on ufw hosts) and one **REAL Major** (any-address jump not followed), both reproduced in this session and both **UNTESTED**. `dotf spec archive` is **not** advisable in this state.

### Recommended next steps

Minimum set to flip to PASS (code + tests first, then the contract-set edits, then a re-review — the next round is the mechanism, not an inconvenience):

1. **code** — `withUFWIptables` must carry `chainUnread` (and any future unknown-read field) from the folded iptables read; a table-driven assertion that every unknown-ness field survives the fold would stop the next field from being forgotten.
2. **tests** — add a ufw-host check row and a ufw-host setup row: `iptables -S fw-open` exits 4, the step is `unverified`, `firewall_ok` false, and the message names the failed read. This is the named regression test the Blocker needs.
3. **code** — make the jump predicate treat `0.0.0.0/0` (and `-d 0.0.0.0/0`) as narrowing nothing, as `::/0` already is for v6; either give the v4 walk its own scope literal or teach the shared predicate both literals and rename the `ip6*` helpers (Minor finding).
4. **tests** — a `TestCheckForLinux` row and a walk-level row for the `0.0.0.0/0` jump (exposure reported, chain read recorded).
5. **spec artifacts (`verification.md` only, outside the contract set)** — record the two new rows against AC1/AC3 and re-state the e2e run; note that AC4's 135-pass figure is carried forward from `c4d8077` and is UNVERIFIED in this review.
6. **spec artifacts (contract set — contract edits belong to this FAIL round and will be re-measured by the next review)** — tick `tasks.md` Closing against `verification.md` (and record `gosec ./...` there or drop it from the task), and give the third acceptance bullet its own number in `proposal.md` so tasks and tests can cite it.

UNVERIFIED in this review, stated rather than assumed:

- **AC4 `make e2e-linux`** (verification.md: "135 pass, 0 fail", and `features.json` f3). Driving `dotf vm` Linux guests is out of reach for a review run; the e2e case itself reads correctly and its assertions (`manual`, the chain-qualified rule name, `firewall_ok` false, the rule surviving setup, `ok` after removal) match AC1/AC4. The case is also not re-runnable on a dirty guest (`iptables -N tsbe2e4` fails if an aborted run left the chain).
- **`gosec ./...`** (listed in the `tasks.md` Closing box, absent from `verification.md`) — the binary is not installed in this environment, so neither the claim nor its absence could be checked here.
- **PR/CI state** — not inspected; this review judges the diff at `a4e0f41`, not the PR's checks.
