---
spec: "SEC-631-scoped-firewall-rules"
verdict: "FAIL"
reviewed_sha: "e6fbbcbd525609327026d529deadbd8cec9db53a"
reviewer: "nan/mimo-v2.6-flash"
date: "2026-11-10"
---

## Adversarial review

**Scope**: SEC-631-scoped-firewall-rules — `git diff 9ec0c909fc8d194839b3f7bebade48ba366e29cb...HEAD`
(16 first-parent commits; this spec's are `80108b2` ADR-017 Amendment B, `605e5d2` iptables,
`6d53dd6` ufw, `55cfd68` Windows, plus round-1-fix PR 5: `af7a330` off-tailnet fix, `27ccaf9`
garbled group count + fresh-guest pin, `12ea1b5` spec reconciliation/AC7, `e6fbbcb` disposition.
The rest arrived through merges from master (#674/#676/#675/#677/#690/#698/#696/#681) and are
judged only where they interact, as in round 1). Round 2: the whole change again, with round 1's
findings re-checked against the late fixes they interact with.
**Sources**: `specs/SEC-631-scoped-firewall-rules/{proposal,tasks,verification,features.json}`,
`review-round1.md`, `internal/host/{linux_steps,windows_steps}.go` + tests,
`scripts/e2e/{linux.sh,windows.ps1}`, `docs/adr/adr-017-host-desired-state.md`.
`contract_digests` in `review-request.json` match `dotf spec digest SEC-631-scoped-firewall-rules`
(identical four hashes), so the contract set is the one this review reads.

### What I ran (fresh, this session, at `e6fbbcb`)

| Command | Result |
|---|---|
| `go test -race -count=1 ./...` | exit 0, 17 packages ok, 0 failures |
| `golangci-lint run ./...` (pinned 2.14.0) | `0 issues.` |
| `gofmt -l .` / `go vet ./...` / `GOOS=windows go vet ./internal/host` / `GOOS=linux go vet ./internal/host` | all clean |
| `bash scripts/check-features-selectors.sh` | `OK (46 go test -run selectors)` |
| `bash scripts/check-features-shape.sh` | `OK (39 features.json files)` |
| features f1 verification (grep on ADR-017) | exit 0 |
| `bash -n scripts/e2e/linux.sh` | syntax OK |
| Mutation A: `offTailnetOnly` forced `false` | **killed** — `TestOffTailnetOnlyAllow` (all 4 subtests) and `TestCheckForLinux/ufw_host,_the_only_allow_is_scoped_off_the_tailnet_(#631)` fail |
| Mutation B: drop the `parseErr` guard in `builtinGroupStep` | **killed** — `TestWindowsBuiltinGroup` "a garbled count…" and "a non-numeric count…" fail |
| Probes (see findings) | run in a `mktemp -d` copy of the tree (`/tmp/sec631probe.*`); repo tree untouched apart from this file |
| `make e2e-linux` / `make e2e-windows` | **not run** — needs `dotf vm` guests; AC6 runs are UNVERIFIED in this session |

`git status --porcelain` shows no changed file outside `specs/SEC-631-scoped-firewall-rules/`.

### Spec and task alignment

- **AC1** (ADR-017 Amendment B): present (`## Amendment B (Accepted, 2026-11-06)`, §5 table cells
  for Linux and Windows); f1's own grep passes. ✔ verified.
- **AC2/AC4** (iptables): scoped append, printed form, two-tier replacement, failed-add/failed-delete
  all exist with named tests (`TestIptablesPrintedForm`, `TestReadIptablesUnscoped`,
  `TestSetupForLinux`, `TestCheckForLinux`), all green fresh. ✔ verified.
- **AC3** (ufw): single write helper pinned by `TestUFWAllowArgs`; legacy/foreign split by
  `TestReadUFWUnscoped`. ✔ verified.
- **AC5** (Windows): scoped recreate, `RemoteAddress` set comparison with dotted-mask/IPv6-case
  normalisation, create-before-remove with terminating errors, built-in group reported and never
  disabled — `TestWindowsFirewallRecreateScoped`, `TestWindowsFirewallRule`,
  `TestWindowsFirewallRecreateOrder`, `TestWindowsBuiltinGroup`. ✔ verified at unit level.
- **AC6** (e2e both OSes): the cases exist in both scripts (off-tailnet cases on both Linux
  backends; Windows scoped-address, fresh-guest group, group enable/disable and legacy-drift
  cases). The runs themselves are UNVERIFIED here (no guests).
- **AC7** (round-1 finding 1): `offTailnetOnly` is consulted in `firewallSetupStep`,
  `firewallCheckStep` and `Service.FirewallOK`; four unit subtests, a `TestCheckForLinux` row and
  four e2e cases cover it, and Mutation A kills the fix's removal. ✔ verified on the paths
  tested — with one untested state where it does not hold (finding 1).
- **Round-1 dispositions**: finding 3 (Windows-e2e status) now agrees across proposal Risks / AC6 /
  tasks / verification / f12 — checked each file. Finding 4: `tasks.md` Closing is fully ticked and
  `verification.md` has the "Promotion candidates" section. Finding 5: parse guard present and
  pinned (Mutation B kills it). Finding 6: the assumption comment on `iptUnscopedRead` is present.
  Round-1 finding 2 (pristine Windows guest): the disposition rests on two e2e cases reading the
  group before setup touches it; script verified, the run itself UNVERIFIED here.
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags in the spec files (only round 1's sentence
  saying none remain, and this transcript).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location |
|----------|---------|------|---------|----------|---------------------------|--------------|
| Major | REAL | correctness / AC7 | **Inactive ufw whose only allow is scoped off the tailnet**: `ufwInactiveState`'s `read.ufwAllows` branch wins before any off-tailnet consideration, so `setup` writes **no** `tailscale0` allow (it does write one in every other non-admitting inactive state: "no allow yet", and it replaces the legacy one) and both commands report `manual` with the message *"its allow is in place for when ufw is enabled"* — an allow scoped to e.g. `10.0.0.0/8`, which will **not** admit the tailnet once ufw is enabled. AC7 says "setup adds the `tailscale0` rule"; here it does not, and check does not say `changed` either. The safety half holds: `firewall_ok` is `false` (reproduced), so this is a wrong-remedy/delayed-convergence defect, not a false convergence. | Probe in a `mktemp -d` copy of the tree: inactive ufw (`Status: inactive`), `ufw show added` = `ufw allow from 10.0.0.0/8 to any port 3389 proto tcp`: `setup outcome=manual msg="ufw is inactive, so it filters nothing: … its allow is in place for when ufw is enabled"` with **no** `run ufw allow …` in `calls`; `check outcome=manual` same message; full `checkLinux`: step "Firewall configuration"=manual, `firewall_ok=false`. Control probe (active ufw, same allow): `setup outcome=changed`, writes `ufw allow in on tailscale0 …` — so only the inactive path misses it. | **UNTESTED** — `TestOffTailnetOnlyAllow` builds only *active* ufw runners (`ufwActive`), and `TestUFWInactive`'s table has no off-tailnet row | code + tests (a `TestUFWInactive` row: off-tailnet-only allow → setup writes the scoped allow and the message says the existing one does not admit the tailnet; plus an e2e case). If the owner accepts the current behaviour instead, that is a **contract edit** to `proposal.md` (its "ufw inactive" fork), needing this review redone |
| Major | THEORETICAL | correctness (iptables) | A **negated-scope** allow (`-A INPUT ! -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT`, i.e. accepts everywhere *except* the tailnet) is classified as the scoped rule: it is neither `iptUnscoped` (has scope) nor `iptAcceptOffTailnet` (the `-i` *value* equals `tailscale0`), so it survives into `rest` and makes `scoped=true` → check and setup both report `ok` with `firewall_ok` true while the tailnet cannot reach the port — the same false-convergence class round 1 blocked, via negation. No known producer: tailscaled writes only `-i tailscale0` accepts (its `! -i tailscale0` lines are DROP/RETURN, and require `-j ACCEPT` here), so no observed host. | Probe (same temp copy): `iptables -S INPUT` = `iptEmpty` + the negated rule → `read.state=rulePresent scoped=true foreign=[]`, `setup outcome=ok "Firewall allows TCP 3389 (iptables)"`, `check outcome=ok "Firewall allows port 3389"` | UNTESTED | code + tests (treat a negated scope as off-tailnet, or a comment on `iptAcceptOffTailnet` recording the assumption, as was done for `iptUnscopedRead`). Follow-up ticket acceptable |
| Question | — | spec artifacts | `proposal.md` still shows every AC box `[ ]` and frontmatter `status: implementing` while `tasks.md`/`verification.md` are complete and ticked. Archived specs are mixed on ticking AC boxes, so this may be exactly what the archive expects; confirm rather than assume (the gate's implementation is out of scope here, so I could not check it — `dotf spec check` is not a subcommand). | file read: `specs/SEC-631-scoped-firewall-rules/proposal.md` | n/a (artifact) | spec (only if the gate wants it; then contract edit + re-review) |

Verification-evidence note (not a finding): the recorded e2e runs (56 / 77 / 84 Linux, 50 / 52
Windows, with log file names) are **UNVERIFIED** in this session — `make e2e-linux`/`e2e-windows`
need `dotf vm` guests I did not spawn. The scripts' cases exist, parse (`bash -n`) and match the
claims' wording.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | Every enumerated AC holds on the paths tested, but AC7 does not hold in the inactive-ufw state (finding 1), which the proposal itself enumerates as a fork — criteria partially met. |
| Verification       | B | Fresh full `-race` run, lint/vet/gofmt, selector guard, two mutants killed and probes reproduce both the fix and its gap; the e2e runs (AC6) are recorded but not reproducible in this session. |
| Scope              | B | The branch's commits match `tasks.md` (four PRs + review-fix PR); the range also carries merged master work, judged only where it interacts. |
| Reliability        | B | Failed add deletes nothing, create-before-remove, parse failures read `unverified`, group never disabled — but one backend state gets a wrong remedy message and no write (finding 1). |
| Maintainability    | B | `golangci-lint` 0 issues; helpers small and single-purpose; several functions carry `//nolint:gocyclo // baseline #629`, so complexity is baselined rather than reduced. |
| Handoff-readiness  | A | All six round-1 findings dispositioned in `verification.md` with red-first evidence, ADR amendment landed, spec files reconciled, promotion candidates recorded. |

### Verdict
FAIL

One **REAL Major** (finding 1): reproduced in a probe against the exact code under review, in a
state the proposal itself discusses, with **no covering test** (`TestUFWInactive` has no
off-tailnet row). AC7 — the criterion added to close round 1 — does not hold there: setup writes
no scoped allow and the message steers the operator to a wrong conclusion. Per
`severity × reality`, a REAL Major requires a fix and a re-review; the rubric's C in
Correctness independently floors the verdict at PASS WITH GAPS, and the REAL Major lifts it to FAIL.
The THEORETICAL negation finding is tracked and does not by itself move the verdict.

### Recommended next steps

For the implementer to apply and record the disposition in `verification.md`. On a FAIL the
contract-set edits are on the table (a re-review follows either way); these are ordered by what
flips the verdict:

1. **Finding 1 (code + tests, or contract).** In `ufwInactiveSetupStep`/`ufwInactiveState`, when
   ufw is inactive and no allow admits the tailnet (only off-tailnet allows, no `ufwScoped`), write
   the scoped allow "for when ufw is enabled" — exactly as the "no allow yet" branch already does —
   and reword the message so it does not claim an off-tailnet allow is "in place" for the operator's
   purpose. Add a `TestUFWInactive` row (setup writes `ufw allow in on tailscale0 …`, check's
   message names the off-tailnet allow) and, ideally, one e2e case. Alternative: if the owner
   accepts today's behaviour, record it in `proposal.md`'s "ufw inactive" fork instead — that is a
   contract edit and the review is redone regardless.
2. **Finding 2 (code + test, or comment; follow-up ticket acceptable).** Make a negated scope
   (`!`, or `-i`/`-s`/`-d` under a `!`) count as off-tailnet in `iptAcceptOffTailnet`, or add a
   comment on that function recording the assumption the way `iptUnscopedRead` now does; pin with
   one `TestOffTailnetOnlyAllow` subtest either way.
3. **Question.** Confirm whether the archive expects the proposal's AC boxes/status updated; if it
   does, that is a contract-set touch to make with the round-3 edits, not after them.

**`dotf spec archive` is not advisable in the current state**: the verdict is FAIL. Minimum set to
flip it to PASS: fix finding 1 (code + a named regression test — or the contract-recorded owner
acceptance), then a fresh re-review; finding 2 and the question can ride along as tracked
dispositions.
