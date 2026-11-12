---
spec: "BUG-736-negated-portless-exposure"
verdict: "FAIL"
reviewed_sha: "6093ab902c51b0eec80fbb9a26c7b8e552c47814"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: `BUG-736-negated-portless-exposure` — `git diff 46b921c1a5c914f517582599bbda05c30386d59b...HEAD` (20 first-parent commits, 13 of them this spec's; 749 insertions across `internal/host/linux_steps.go`, its test file, `scripts/e2e/linux.sh`, ADR-017 / `docs/design/host-client-parity.md` / `lesson-055`, and the spec folder). This round reads the WHOLE change, and judges the late fixes (`584fcb1`, `1db3d63`, `8b93885`, `6093ab9`) against the early ones they interact with (`96247de`, `b6a6c7b`, and the `iptExposes` predicate itself).
**Sources**: `specs/BUG-736-negated-portless-exposure/{proposal,tasks,verification}.md`, `features.json`, `review-request.json` (contract digests re-checked with `dotf spec digest` — all four match), the round-1/2/3 review copies, issue #743 and #745 re-read live, the full diff, and two probe suites run and deleted in this worktree at `6093ab902c51b0eec80fbb9a26c7b8e552c47814`.

### What was verified by running it

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go test -race ./... -timeout 8m` | every package `ok` (19 packages incl. `internal/host`), 0 failures |
| `golangci-lint run ./internal/host/...` (2.14.0, mise pin) | `0 issues` |
| `GOOS=linux golangci-lint run ./internal/host/...` | `0 issues` |
| `GOOS=linux go vet ./internal/host/` | clean |
| `gofmt -l .` | empty |
| `shellcheck -s sh scripts/e2e/linux.sh` | clean (new e2e block included) |
| `bash scripts/tests/test-e2e-report.sh` | all cases passed (the CI shim suite; no shim reads `linux.sh` directly) |
| `dotf spec digest BUG-736-negated-portless-exposure` | identical to `review-request.json`'s `contract_digests` |

**Mutation red-green, both reverted (tree left clean):**

- Removed `legacyHeld`'s new `ruleAbsent` branch (round-3's fix): `--- FAIL: TestSetupForLinux/..._is_the_only_allow:_scoped_rule_added,_manual_(#736_review_round_3)` (`outcomes = [ok changed ok], want [ok manual ok]`) and `--- FAIL: TestCheckForLinux/..._is_the_only_allow:_manual_(#736_review_round_3)`. The round-3 Blocker is genuinely pinned.
- Reduced `iptExposes` to `iptPortCovers` (dropping the negated-port branch): `TestExposureShapes` fails on four rows, plus `TestReadIptablesUnscoped/an_accept_for_every_port_but_SSH_is_foreign` and `TestIP6Exposure/a_v6_accept_for_every_port_but_SSH`. The core predicate is pinned on both reads.

**Full-path probes through production code** (`readLinuxFirewall` → `firewallCheckStep` / `firewallSetupStep`, `recordingRunner` supplying only command output; both scratch files deleted afterwards). Twenty-one shapes tried: only-allow negated port; only-allow negated loopback; only-allow portless `-p tcp`; multiport negated leaving the port out and including it; negated dport + `-m state --new`; negated dport + `--comment`; negated dport + `! --ctstate INVALID`, `! -f`, `! --syn`; scoped `-i eth0` + negated dport; `! --dport 3389` as the only allow; negated dport beside a v6 portless allow; the two #743 residual mechanisms; a ufw-active host with only the iptables negated allow; and **positive source-port matches** (`--sport 1024:65535`, `--sport 1:65535`). Highlights:

- **Round-3's Blocker is fixed**: `-P INPUT DROP` + `-A INPUT -p tcp -m tcp ! --dport 22 -j ACCEPT` as the only allow → `check: manual`, naming the rule, `firewall_ok=false`; `setup: manual`, whose only mutation is `run iptables -A INPUT -i tailscale0 -p tcp --dport 3389 -j ACCEPT` — **no `-D`** — with the SSH caution in the fix.
- Only-allow negated loopback and only-allow portless `-p tcp` read the same way; a re-run on a host already holding the scoped rule mutates nothing and stays `manual`.
- `! --dports 22,3389` for port 3389 is correctly **not** reported (the negation excludes the port), and `! --dport 3389` as the only allow correctly falls to `changed` + the scoped rule.
- **The finding below**: a rule whose only port match is a *positive* source range reads `ok` / `firewall_ok: true` even when that range covers every port.

**Not verified here**: AC4's VM run (`make e2e-linux`, claimed 108/0 at `8b93885`) cannot be reproduced in a worktree — the harness drives Lima guests through `dotf vm`. The script block is shellcheck-clean; the run itself is **UNVERIFIED** by this review.

### Spec and task alignment

- `proposal.md` "What" and the code agree on the mechanism: `iptExposes(f, port)` serves both reads (`iptUnscoped` v4 INPUT, `ip6Exposed` v6 INPUT); `iptPortCovers` stays the "names the port" half; `iptPortlessCovers` covers the no-port-flag half; `iptablesDecide` is untouched, as declared; `iptScopedAllow` replaces `iptHasScope` in the allow read only, with the deny path keeping `iptHasScope` (declared in `verification.md`).
- `keepOpenFirst` is shared by `withPolicy` (#691), `foreignFix` and `withIP6`; `iptAdmitsOthers` keeps the SSH caution off ufw lines (round-1 F1) and off rules naming a positive destination port (#745).
- `tasks.md` ticks all carry diff evidence; `verification.md` records AC1–AC3 with commit shas and named tests, plus the residual with both mechanisms after round-3 F2. Contract digests match; no `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags remain.
- ADR-017 (Amendment B text), `docs/design/host-client-parity.md` and `lesson-055` are amended in place, not duplicated — correct knowledge placement, and no vault artifact is owed for an internal firewall read.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | exposure predicate (both reads), AC3 | **A positive source-port match is read as narrowing, so a source range that covers every port reads `ok`.** `iptExposes` sends any port-bearing rule to `iptPortCovers \|\| iptExcludesOthers`, and `iptExcludesOthers`'s no-destination branch demands that **every** source-port match be *negated* — so `-A INPUT -p tcp -m tcp --sport 1:65535 -j ACCEPT` is not reported. A source port is the client's ephemeral port, always in `1..65535`, so that rule is **exactly equivalent** to `-A INPUT -p tcp -j ACCEPT`, which this same change now reports `manual`. Beside the scoped rule the exposure vanishes entirely: `check: ok`, `firewall_ok: true`. `--sport 1024:65535` — a shape real iptables templates use to approximate "return traffic" — behaves the same, while admitting the port from nearly every client (ephemeral ports start at 32768 on Linux, 1024 on Windows). This contradicts the rationale the change itself adopted in round-1 F2 ("a source-port match does not narrow, since clients connect from ephemeral ports") and applied to the *negated* polarity only; it is the same silent-exposure class as #691/#736, left inside the predicate this change rewrites. The code matches AC3's clause ("`--sport`-only rules" not reported), so the **contract clause is what is defective**, not the implementation's adherence to it. | Probes (production path, `iptables -S` output simulated, scoped rule present): `-A INPUT -p tcp -m tcp --sport 1:65535 -j ACCEPT` → `state=present scoped=true foreign=[] ok=true`, `check: ok`; `--sport 1024:65535` → identical; the same two rules as the *only* allow → `state=absent foreign=[] ok=false`, `check: changed "No firewall rule found for port 3389"`, so the rule is invisible in both placements. Contrast, same run: `-A INPUT -p tcp -j ACCEPT` → `foreign=[…] ok=false`, `check: manual`. | UNTESTED — `TestExposureShapes` has no positive-source-port row at all; the only sport rows are `! --sport 22` (true) and `--sport 22` (false). | code (`iptExposes`/`iptExcludesOthers`: with no destination match, a positive source-port match does not prove non-exposure — report it, the fail-closed direction this repo uses elsewhere) + tests (rows for `--sport 1:65535`, `--sport 1024:65535`, and a narrow `--sport 22` if you decide it stays unreported **for a stated reason**) + spec (`proposal.md` "What" and AC3's "`--sport`-only" clause). Contract set changes ⇒ re-review. |
| Major | REAL | v4/v6 symmetry (AC1 vs AC2) | A **negated non-tailnet scope** still reads as the operator's scope on v4, so a host exposing the port through `--dport 3389 ! -s 100.64.0.0/10` (or portless `! -i tailscale0 -j ACCEPT`) reads `ok` with `firewall_ok: true` on v4 while the v6 read reports the same rule. **Pre-existing, unchanged by this diff, disclosed and deferred**: `proposal.md` Risks names it as "the decision #743 asks for", #743 is **OPEN** and its body names both mechanisms and the fix direction; the probe reproduces both (`foreign=[] ok=true` for each). Same mechanism covers any other negation — `! --ctstate INVALID`, `! -f`, `! --syn` beside `! --dport 22` all read `foreign=[] ok=true` (probes), because `iptScopedAllow` treats the bare `!` token as a scope flag. Disposition: **deferred to #743, not escalated** — outside this spec's declared scope, not introduced or widened here (old `iptHasScope` also counted `!` as a scope), and the fix requires the v4 semantic decision that ticket owns. | Probes above; `gh issue view 743` → `"state":"OPEN"`, "fix(host): report a port-less allow under a negated scope as exposing the port". | `TestReadIptablesUnscoped/{"a negated source does not admit the tailnet"}` and `{"/a negated tailnet interface …"}` pin the v4 non-reporting; `TestIP6Exposure` + `lesson-055` pin v6 reporting. No test asserts cross-read agreement — UNTESTED by design until #743 decides it. | code (align v4 with v6) in #743; spec note only for this spec. |
| Minor | REAL | AC wording vs code | AC3 lists "`--sport`-only rules" among shapes **not** reported, but a rule whose only port match is a **negated** source port **is** reported and pinned as reported (`TestExposureShapes` row `! --sport 22` → `true`). The proposal's own "What" says so ("with no destination match, when every source-port match is negated"), so AC3's list is the sloppy half. Read alone, AC3 scores this change as failing its own criterion. | `TestExposureShapes` rows `! --sport 22` (true) vs `--sport 22` (false); "What" third bullet. | `TestExposureShapes/-A_INPUT_-p_tcp_-m_tcp_!_--sport_22_-j_ACCEPT` | spec (`verification.md` reading, or a follow-up ticket — this one can wait for the same edit pass as the Major above). |
| Minor | REAL | handoff artifacts | The round-3 review copy and request — `review-round3.md`, `review-request-round3.json` — exist in the tree but are **untracked**, while rounds 1 and 2 are committed (`664354f` set the precedent). Committing only `review.md` drops the round-3 provenance from the archive. | `git ls-files specs/BUG-736-negated-portless-exposure/` lists rounds 1–2; `ls` shows round 3 present but untracked. | n/a (artifact hygiene) | spec artifacts — commit both beside `review.md`. Not in the contract set; free to change. |
| Minor | REAL | fix text (pre-existing) | A foreign rule that **also names SSH** (`-A INPUT -p tcp -m multiport --dports 22,3389 -j ACCEPT`) is reported `manual` with "scope … or remove it" and **no** loopback/SSH caution, though removing it closes SSH under a DROP policy. Declared in Risks, ticketed. Disposition: **#745 (OPEN, verified this session)**. | `TestWideAllowFix` pins the absence; `gh issue view 745` → `"state":"OPEN"`. | `TestWideAllowFix` (passes as written) | code + tests in #745. |
| Minor | THEORETICAL | v4 scope read | `-A INPUT -p tcp -d 0.0.0.0/0 -j ACCEPT` reads as a scope on v4 (no `::/0` analogue), so `check` would say "every allow is scoped to another interface or address" for a rule that narrows nothing. `iptables -S` omits a default destination, so reachability is unproven; declared in Risks after round 3 (F4). Surface only. | No `-d 0.0.0.0/0` row in `TestExposureShapes`; `iptPortlessToken` treats `-d` as non-narrowing and `iptScopedAllow` then hides it. | UNTESTED | spec (already declared) — do not gate. |
| Question | SPECULATIVE | parser coverage | `-A INPUT -m tcp ! --dport 22 -j ACCEPT` (negated destination port, no `-p tcp`) is not reported, since `iptExcludesOthers` requires `-p tcp`. `iptables` refuses `--dport` without a protocol match and `iptables -S` reprints `-p tcp -m tcp`, so the shape is unreachable in practice. Recorded so the requirement is written down rather than assumed. | Probe: `foreign=[]` → `check: changed`. | UNTESTED | none — no action proposed. |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness | **C** | AC1/AC2/AC3/AC4 shapes are backed by tests I ran and re-proved by mutation, but the predicate misses a shape equivalent to the portless rule it now reports, and beside the scoped rule that shape reads `ok` with `firewall_ok: true` — a negative-path gap in the very class of bug the spec exists to close. |
| Verification | B | Commit-pinned and reproducible for the unit/lint layer (suite, both linters, vet, gofmt re-run here); AC4 is stated as a VM run rather than dressed up as local, and is not reproducible in a worktree. |
| Scope | A | One predicate, one shared fix helper, two reads, docs amended in place; no dependency, no unrelated change, `iptablesDecide` deliberately untouched. |
| Reliability | B | Setup refuses to delete foreign rules in every probed shape, appends only the scoped rule and is idempotent on re-run; failure paths unchanged and tested. |
| Maintainability | A | New functions are 4–14 lines with low branching (`iptExposes` 7, `iptScopedAllow` 7, `iptOpenNegation` 10, `iptExcludesOthers` 14, `iptExcludes` 6, `iptAdmitsOthers` 6, `keepOpenFirst` 4); comments state WHY; linters and vet clean on both GOOS targets. |
| Handoff-readiness | B | Spec, ADR-017, parity doc and `lesson-055` updated in place, residuals linked to OPEN tickets (#743, #745 verified); only the round-3 review copy is untracked. |

### Verdict

**FAIL.**

- There is no Blocker. The **Major REAL** at the top of the findings table is one: a rule that is logically identical to a shape this change now reports (`--sport 1:65535` ≡ `-p tcp -j ACCEPT`) is read as non-exposing, and beside the scoped rule it produces `firewall_ok: true` while every client can reach the port. It is reproduced in this session, so it is not a hypothetical, and it lands in the predicate this change rewrites — the fix is small (treat a positive source-port match with no destination match as not proving non-exposure, add the rows, and state the decision), and the contract clause that currently blesses it (`AC3`'s "`--sport`-only rules") has to change with it, which is what a re-review after the fix is for.
- The second **Major REAL** (v4/v6 asymmetry on negated scopes) is deliberately **not** escalated: it predates this change, is disclosed in `proposal.md` Risks, its mechanism now matches an OPEN ticket's own description (#743), and the fix requires the v4 semantic decision that ticket owns. It is recorded with that disposition, which is the room the severity × reality rule leaves for a tracked, unchanged item.
- Rubric path: one C, no D.

### Recommended next steps

The contract set is in play on a FAIL, so these are the edits plus a fresh round — in this order:

1. **Fix the source-port polarity in `iptExposes`/`iptExcludesOthers`** (code): with no destination port match, a positive source-port match must not, by itself, prove the rule narrows — the argument the change already accepted in round-1 F2. Reporting it is the fail-closed direction this file uses elsewhere (`iptPortlessCovers`'s "fails closed toward narrowed" is about *unmodelled* tokens, not about a modelled match that narrows nothing).
2. **Tests first** (red): `TestExposureShapes` rows for `--sport 1:65535` and `--sport 1024:65535` → `true`, and a step-level row that a host with the scoped rule plus `--sport 1024:65535` reads `manual` with `firewall_ok` false — the probe shape that currently reads `ok`. Keep or change the existing `--sport 22` → `false` row **by decision**, and say which.
3. **Contract edit** (`proposal.md` "What" + AC3, `tasks.md` row, `verification.md` evidence): make the `--sport`-only clause say what is reported and what is not, and why. This invalidates the current review by design; the next round reads the new digest.
4. **Have the next round check #743's two mechanisms**; leave that Major deferred unless the owner decides to fold it in here, and keep #745 open for the `--dports 22,3389` caution.
5. **Commit `review-round3.md` and `review-request-round3.json`** beside `review.md`, so the archive carries the round-1–3 provenance.
6. **AC4**: attach the e2e output to `verification.md` or accept the record as the implementer's capture — this review could not reproduce the VM run.

`dotf spec archive` is **not advisable** in the current state: the verdict is FAIL and the source-port gap is inside the acceptance criteria this spec exists to satisfy.
