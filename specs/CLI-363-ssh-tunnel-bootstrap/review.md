---
spec: "CLI-363-ssh-tunnel-bootstrap"
verdict: "FAIL"
reviewed_sha: "8020bf5a86b02bfd85a08c1d968b99809496b222"
reviewer: "nan/mimo-v2.5"
date: "2026-09-29"
---

## Adversarial review — round 2

**Scope**: `CLI-363-ssh-tunnel-bootstrap` — re-review of the round-1 fix commit
`8020bf5 test(connect): close SSH bootstrap review gaps` on top of the merged
feature (`ad9b8b1` #372 → `1c5a966` #373 → `54c3b91` #374).
**Sources**: `specs/CLI-363-ssh-tunnel-bootstrap/{proposal,tasks,verification,features}.md`,
round-1 `review.md` (verdict FAIL at `5abc6c7`), `git diff 1c5a966..8020bf5`,
`git show 8020bf5`, `git log 1c5a966..HEAD`.

Round 1 failed on two REAL Majors (F1: proxy-install seam unpinned; F2:
shutdown closer unpinned) and listed eight Minors. This round (a) re-ran the
whole verification independently, (b) confirmed the round-1 fixes with a fresh
mutation battery, and (c) pushed the battery one layer deeper into `run()` and
`ConfigureControlProxy`, where round 1's recommendations pointed and were only
partially applied.

### Verification performed in this session

```
go build ./...                                    OK
go vet ./...                                      clean
go test ./...                                     13/13 packages ok
golangci-lint run                                 0 issues (pinned v2.12.2)
gosec ./internal/bootstrapssh/... ./cmd/cli/...   0 issues (16 files, 7 nosec)
features.json verification f1..f4 (all four)      pass, non-vacuous (named tests run)
contract digests vs review-request.json           3/3 MATCH (dotf normalisation replicated:
                                                   CRLF, checkbox fold, frontmatter status,
                                                   features state/evidence stripped)
review.md digest vs review_digest_before          MATCH — round-1 review untouched
```

No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags in any spec file. Working tree
left clean; every mutation below was reverted (`git checkout --`).

### Mutation battery (each applied → targeted package test → reverted)

| # | Mutation | Result |
|---|---|---|
| M1 | Remove the `controlProxyConfigurer(...)` call in `startConfiguredBootstrap` (`cmd/cli/run.go`) | **FAIL `cmd/cli`** — round-1 F1 seam is now pinned |
| M3 | Replace `startBootstrapLifecycle`'s closer with `func() {}` | **FAIL `cmd/cli`** — round-1 F2 closer is now pinned |
| M5 | Revert the `bootstrap_socks_addr` ↔ `socks5_addr`/`local_addr` collision check (`internal/config/merge.go`) | **FAIL `internal/config`** — round-1 Minor applied and pinned |
| M4 | Drop `closeBootstrap()` from `run()`'s own defer (keep compile-valid) | **GREEN** — `run()` never proves it invokes the closer |
| M6 | Drop `cancelWithCause(nil)` before the close in `run()`'s defer (round-1 shutdown-ordering Minor) | **GREEN** — the ordering fix is unpinned |
| M7 | Remove the final `emitRunCause` call site at the end of `run()` | **GREEN** — round 1 recorded this mutation as FAIL; it does **not** reproduce at HEAD (identical code block, only tests were added since — the round-1 FAIL was most plausibly a build-breaking variant, which is not a valid mutation) |
| M2‴ | Replace the SOCKS hook install inside `ConfigureControlProxy` with a pass-through `SetProxyFunc` (compile-valid) | **GREEN** — the real hook-install body has no test |

### Spec and task alignment

- **AC1 — met, pinned.** Precedence (`flags > env > YAML`), loopback
  validation, and control-URL requirement verified by round 1 and re-run here
  (`TestMergeBootstrapSSHPrecedence`, `TestMergeRejectsInvalidBootstrapConfig`,
  `TestMergeRejectsBootstrapListenerCollision`, `TestCollectFlagsBootstrapSSH`,
  `TestBootstrapSSHFlagsReachRunner` — all pass, f1 command green).
- **AC2 — met in code; coverage partially applied (F1).** The call-site seam is
  now pinned (M1 red). Round-1 rec #1 (`TestConfigureControlProxyInstallsTheHook`
  against the real function) was **not applied** — every variant round 1 offered
  included it. `TestControlProxy*` cover only the unexported factory
  `controlProxyFunc`; M2‴ shows a regression in the install body still ships
  green, silently degrading the feature to a direct dial — F1's own failure
  mode. Feasibility check done: no test in `internal/bootstrapssh` touches
  `tshttpproxy.ProxyFromEnvironment`, so a single direct test (call
  `ConfigureControlProxy`, assert `ProxyFromEnvironment` returns
  `socks5h://…` for the control URL and env fallback otherwise) fits the
  package without order clashes.
- **AC3 — met at factory level; run()-level half still unpinned (F2 partial).**
  `TestStartBootstrapLifecycleCloserClosesTunnel` turns M3 red, but round-1
  rec #3 asked for a `run()`-level test and the delivered test stops at the
  factory: M4 (the actual production defer line), M6, and M7 all stay green.
  The only `Run()` test (`TestRunBootstrapFailureStopsBeforeTailscale`) returns
  before any of those lines execute. Startup failures + false-`READY` guard
  remain properly pinned (`TestStartReports*`, `TestEmitReadyIfActiveSuppressesCanceledStartup`).
- **AC4 — met.** `TestBootstrapSSHDocumentation` pins `.env.example`, README,
  site config/CLI reference, ADR-005 and lesson-014; the ADR gained the
  unauthenticated-SOCKS-pivot note round 1 asked for (M5/M6-side Minors:
  collision check ✅ applied, ADR doc ✅ applied; ordering fix applied in code
  but unpinned per M6).
- **Round-1 declined items** (hostname-only selection, process-global hook,
  TLS wording, separate ADR) each carry an explicit rationale in
  `verification.md` — dispositions recorded, accepted as-is.
- `tasks.md` closing box "Every acceptance criterion … covered by at least one
  test" is **overstated**: AC2's install body and AC3's run()-level shutdown
  have no named test (see findings).
- `features.json` remains `state: pending` / `evidence: ""` — correct
  pre-harness shape; the harness must capture evidence before archive.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location |
|----------|---------|------|---------|----------|---------------------------|--------------|
| Major | REAL | lifecycle / AC3 | `run()`'s shutdown/exit wiring is untestable-by-deletion: dropping `closeBootstrap()` from the defer (M4), the cancel-before-close ordering (M6), or the final `emitRunCause` (M7) all leave the suite green. Round-1 rec #3 asked for a `run()`-level test; the delivered test covers only `startBootstrapLifecycle`. Round 1's own "emitRunCause pinned" claim does not reproduce at HEAD. | Mutation battery above | UNTESTED (closest: `TestStartBootstrapLifecycleCloserClosesTunnel`, factory-level only) | tests |
| Major | REAL | proxy / AC2 | The real `ConfigureControlProxy` body is unpinned: swapping the SOCKS hook for a pass-through (M2‴) is green. Round-1 rec #1 was skipped in all offered variants while `verification.md` claims "F1 — applied". | Mutation M2‴; `grep -rn ConfigureControlProxy --include=*.go` → no test file references it | UNTESTED (`TestControlProxy*` cover the factory only) | tests (+ correct `verification.md` wording) |
| Major | REAL | git hygiene | `8020bf5` carries `Co-authored-by: Copilot <…>` (as does merged `54c3b91`), violating the standing "No AI attribution in git history" order. Precedent is repo-wide (24 such commits across refs, 40 `Co-authored-by` on master), so this is also a policy question for the human — but the unmerged commit is amendable now. | `git log -1 --format=%B 8020bf5` | n/a | git history (amend before push; human call on precedent) |
| Minor | THEORETICAL | wiring | `run()` is only ever exercised up to its first failure branch; any future tail-of-`run()` contract will have the same blind spot until a post-bootstrap `Run()` test exists. | M4/M6/M7 green | UNTESTED | tests (subsumed by Major 1) |
| Minor | THEORETICAL | global state | `tshttpproxy.SetProxyFunc` errors once `config` is initialised (`tshttpproxy.go:46-56`), making direct hook tests order-sensitive — the likely reason rec #1 was skipped. Legitimate constraint, but it needs recording, not silent omission. | Module source read | UNTESTED | tests (one-shot test per binary) / verification.md |
| Minor | THEORETICAL | proxy selection | Hostname-only matching routes same-host different-port requests through SOCKS (round-1 carry-over; declined with rationale recorded). | Round-1 code read | `TestControlProxySelectsOnlyControlHostname` | spec (disposition recorded — accepted) |
| Question | — | process | `features.json` `evidence` is empty; `dotf spec archive` freshness re-computation passes (digests match), but the harness must capture `passing` evidence before archive. | `features.json`; digest check above | — | harness |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All four criteria hold on the code as written; security-sensitive command construction is defensive (no shell, `-l` user, dash/whitespace rejection, host-key defaults intact). |
| Verification       | C | Round-1's named mutations are now red, but two AC-critical layers (hook install, run()-level shutdown/exit) remain mutation-green while `verification.md` claims both findings "applied". |
| Scope              | B | Diff matches the proposal; fix commit touches only tests, the two seams, the collision check, ADR text and verification.md. |
| Reliability        | B | Error paths classified and fail-loud; the residual risk is detection-of-regression, not live misbehaviour. |
| Maintainability    | B | Small functions, good wrapping; package-level test seams match the existing `bootstrapStarter` idiom. |
| Handoff-readiness  | B | Round-1 dispositions recorded, promotions answered, ADR/lesson updated; the "applied" wording overstates and must be corrected with the fixes. |

### Verdict

**FAIL** — two **REAL** Majors on acceptance-criteria coverage, both
mutation-demonstrated and **UNTESTED**, plus one **REAL** Major on git
hygiene. The rubric alone would be PASS WITH GAPS (one C, no D); the severity
axis escalates. Round 1's literal flip condition (named, mutation-sensitive
tests for its F1/F2 mutations) **is met** — M1 and M3 are red — but the
battery shows the fix stopped one layer short of what round 1's
recommendations asked for, and `verification.md` records it as fully applied.

### Recommended next steps (minimum set to flip to PASS)

1. **Run()-level shutdown test** (turns M4 red): inject a fake
   `bootstrapStarter` returning a `fakeControlBootstrap`, force `run()` past
   `startBootstrapLifecycle` to a post-bootstrap return (e.g. unreachable
   `ControlURL` + short `ConnectTimeout` so `initTailscale` fails fast), then
   assert `tunnel.closed`. If the failure path can also assert the
   `ssh_bootstrap_failed` ERROR line, M7 pins too.
2. **Direct hook-install test** (turns M2‴ red): in
   `internal/bootstrapssh`, call the real `ConfigureControlProxy` once, then
   assert `tshttpproxy.ProxyFromEnvironment` returns `socks5h://127.0.0.1:1055`
   for the control URL and the env fallback for an unrelated URL. No other test
   in that binary initialises `config`, so the once-only constraint holds.
3. **Amend `8020bf5`** to drop the `Co-authored-by: Copilot` trailer before
   push; raise the repo-wide precedent (24 commits) as a separate policy
   question rather than silently extending it.
4. **Correct `verification.md`**: change "F1/F2 applied" to match the actual
   state after (1) and (2) land; tick nothing in `tasks.md` that lacks a named
   test.
5. Re-run this review (round 3); expected verdict **PASS** once M2‴ and M4
   are red, with the Minors carried as dispositions.

**Archive advisory**: `dotf spec archive` / `/spec archive` is **not
advisable** in the current state — verdict is FAIL, and `features.json` has
not yet been harness-stamped `passing`.
