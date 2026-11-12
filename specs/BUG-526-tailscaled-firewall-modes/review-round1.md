---
spec: "BUG-526-tailscaled-firewall-modes"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "8d832f228960ee390098cab4e4d70a36a25b7797"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-11"
---

## Adversarial review

**Scope**: `BUG-526-tailscaled-firewall-modes` — whole range
`ad10e1b282b1a97c52fb67634bda366c0b0823ce...HEAD` (`8d832f2`). The branch's own commits are
`edd29a3` (code + unit tests), `65994ad` (e2e), `cf2366c` and `8d832f2` (docs + spec evidence);
`6bc1fba` is a merge of `origin/master` that brought #704 (SEC-631 off-tailnet), #705 (e2e CI) and
#706 (reviewer pool) into the range. Those three are judged only where they interact with this
spec, which they do in `linuxFirewallOK` and in the setup/check outcome order.
**Sources**: `specs/BUG-526-tailscaled-firewall-modes/{proposal,tasks,verification}.md`,
`features.json`, `review-request.json`, the diff above, and re-run evidence (`go test -race ./...`,
`golangci-lint run ./...`, `gosec ./...`, mutant edits in a throwaway clone, probe tests in a
throwaway clone, `scripts/e2e/out/linux-20261111T071101Z.log`).

### Spec and task alignment

- Contract digests in `review-request.json` equal `dotf spec digest
  BUG-526-tailscaled-firewall-modes` exactly (all four values), so the contract is the one the
  launcher recorded. `reviewed_sha` is `git rev-parse HEAD`.
- `tasks.md`: every implementation and closing box is `[x]`, and each ticked box has diff
  evidence — `edd29a3` for AC3/AC4 read + tests, `65994ad` for AC5 e2e, `cf2366c`/`8d832f2` for
  the docs and spec evidence, `b04a956` (pre-base, on master) for the lesson correction.
- AC1/AC2 are pinned by `TestReadIptablesCapturedModes` (subtests `netfilter-mode_nodivert`,
  `nftables_mode,_read_through_iptables-nft`), which exists on the base and passes. AC1's second
  half ("setup writes nothing") is proven by `TestFirewallStepsTailscaledModes/nodivert`, not by
  the test `verification.md` cites — the citation is incomplete, not wrong (finding 5).
- AC3/AC4 are pinned by `TestIP6Exposure`; AC5 by the e2e log. I re-ran the three mutants
  `verification.md` claims and all three fail `TestIP6Exposure` as stated:
  `len(fw.v6Foreign) == 0` dropped from `linuxFirewallOK` → `linuxFirewallOK = true, want false`;
  `withIP6` dropped from the setup step → `outcome ok ..., want manual`; `withIP6` dropped from
  the check step → the same. The mutation claim is honest.
- `verification.md` promotion lines are all answered, the lesson it names
  (`docs/lessons/lesson-055-2026-11-04.md`) exists and carries the nodivert/nftables correction,
  and `docs/design/host-client-parity.md` was updated to match what the code does. No
  `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags remain in the spec.
- Scope of the branch's own work matches the proposal: one 68-line code change, its tests, one
  e2e block, and docs. The merged SEC-631 work is visible in history as a merge, not smuggled in.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | THEORETICAL | exposure scan / correctness | `readIP6` reuses `iptUnscoped` — the pre-#631 "rule shape setup wrote" detector — as the IPv6 **exposure** detector. That detector treats any `-i`/`-s`/`-d` or a bare `!` as scope (`iptScopeFlags = {"-i","-s","-d","!"}`), so a v6 rule that does open the port on every interface is invisible to the scan: `-A INPUT ! -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT` (open on every interface except the tailnet one) and `-A INPUT -s ::/0` / `-d ::/0` (explicit any). `check` then reports `ok` and `firewall_ok: true` on an exposed host, which is exactly the failure the spec was written to remove ("reports a host converged while it is exposed over IPv6"). It is also inconsistent with the v4 read in the same series: `iptAcceptOffTailnet` was hardened after the #631 review to treat a negated scope as off the tailnet, so the same rule shape is `changed`/`firewall_ok:false` in v4 and invisible in v6. No capture or fixture in the repo has such a rule, so this is argued from code and probe, not observed on a host. | Probe in a throwaway clone (`internal/host`, same package): v4 `! -i tailscale0 ...` → `outcome=changed ok=false`; the same rule in `ip6tables -S INPUT` → `outcome=ok ok=true v6foreign=0`; `-s ::/0` and `-d ::/0` also `v6foreign=0`. `iptScopeFlags` at `internal/host/linux_steps.go:610`. | UNTESTED — `TestIP6Exposure` has no negation and no explicit-any case, and `TestReadIptablesUnscoped`'s two negated cases exercise the v4 path only | code (`readIP6`/`iptUnscoped` scope test, or a v6-specific predicate) + tests (a named `TestIP6Exposure` case per shape) |
| Minor | REAL | AC3 wording vs code | "check and setup report `manual`" is not universal. When the v4 outcome is not converged, the v6 finding is reported in the message but the outcome keeps the v4 value: nodivert v4 + an unscoped v6 allow → check `unverified`; a failed v4 write + an unscoped v6 allow → setup `failed`. Both still name the rule and `firewall_ok` is false, so the operator cannot mistake the host for converged — but the AC as written is narrower than the code. | Probe: `nodivert v4 + v6 open: outcome=unverified ok=false msg="Cannot read the firewall: ...; ip6tables admits TCP 3389 on every interface: ..."`; `setup write fails + v6 open: outcome=failed msg="Could not add the firewall rule: ...; ip6tables admits ..."`. Cause: `withIP6` upgrades only `OutcomeOK`/`OutcomeChanged`. | UNTESTED (no `TestIP6Exposure` case combines a non-converged v4 step with a v6 allow) | code (or `proposal.md` AC3 wording — but that is contract-set, so on this verdict it is dispositioned in `verification.md`/a follow-up ticket, not edited now) |
| Minor | REAL | AC4 note quality | A failed v6 read puts the Go error into the step message verbatim (`IPv6 not read: ip6tables -S INPUT: exit status 1`), with no tool stderr, unlike `chainMissing` which quotes the tool's own words. On a host with `ipv6.disable=1` — the case AC4 exists for — every firewall step gains that note and it says nothing about why. | Probe: `failed read: note="ip6tables -S INPUT: exit status 1"`. `readIP6` uses `fmt.Sprintf("%s -S %s: %v", ...)` rather than the `exec.ExitError` stderr path. | UNTESTED (the `v6 read fails` case asserts only that the message contains `ip6tables -S INPUT`) | code |
| Minor | SPECULATIVE | tests / scaffolding reach | `withoutIP6Probe` strips `lookpath ip6tables` from *every* existing call-order assertion in `TestSetupForLinux` and `TestCheckForLinux`, so a duplicated or misplaced probe on the v4 path is no longer detectable there, and `TestIP6Exposure` asserts the exact call sequence for `firewallSetupStep` only — the check path's sequence is unasserted. The covered path is pinned; this is about what the filter stops seeing. | `internal/host/linux_steps_test.go` (`withoutIP6Probe`, and `TestIP6Exposure` asserting `r.calls` after setup only) | `TestIP6Exposure` (partial) | tests |
| Minor | REAL | verification artifact | `verification.md` maps AC1 ("check reports `unverified` naming the mode, **and setup writes nothing**") to `TestReadIptablesCapturedModes/netfilter-mode_nodivert`, which asserts the read only. The "writes nothing" half is proven by `TestFirewallStepsTailscaledModes/nodivert`. The evidence exists; the citation is incomplete. | `TestReadIptablesCapturedModes` reads `readIptables` and checks `state`/`reason`; `TestFirewallStepsTailscaledModes` asserts no `run ` call on setup and `OutcomeFailed`/`OutcomeUnverified`. | `TestFirewallStepsTailscaledModes` | verification.md (outside the contract set, so it can be corrected without invalidating this review) |

Nothing found in the injection / secrets / auth / performance / resilience columns of the
code-level checklist: the diff builds no shell command from read output (rules are printed in a
message string, never executed), adds no credential handling, and `readIP6` is one bounded exec.
Cyclomatic complexity of the new functions is low (`readIP6` ≈4, `withIP6` ≈5, `linuxFirewallOK`
1); the pre-existing `//nolint:gocyclo` on `readIptables` is unchanged baseline.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1–AC5 hold on the paths that were tested and the e2e log reproduces 89 pass / 0 fail; negative-path gap in the new v6 scan (finding 1) and the AC3 outcome edge (finding 2) |
| Verification       | A | Every AC has a named test or a re-run command; I reproduced the suite (`go test -race ./...` exit 0), all three claimed mutants, `golangci-lint run ./...` (0 issues), `GOOS=linux`/`GOOS=windows` vet, `gosec ./...` (no findings) and the e2e log (89 pass, 0 fail, 0 xfail/xpass) |
| Scope              | B | The spec's own diff matches the proposal exactly; the review range also carries the #704/#705/#706 master merge, which is declared in history and interacts with this code in `linuxFirewallOK` |
| Reliability        | B | Failed/absent v6 reads are handled as notes rather than failures, nothing is deleted, and unprivileged reads probe nothing; the false-negative in finding 1 is a reliability gap in a safety scan |
| Maintainability    | A | Small single-purpose functions with why-comments, table-driven tests, no dead code, lint clean |
| Handoff-readiness  | A | Docs updated to match behaviour, verification decisions recorded, promotions answered, lesson on master; one incomplete citation (finding 5) |

### Verdict

PASS WITH GAPS

No Blocker, no REAL Major, no rubric D or C. One Major is THEORETICAL (finding 1) and is tracked
rather than blocking; four Minors are open with a disposition each. `dotf spec archive` is
advisable in this state: the review is fresh for the recorded contract digests (checked with
`dotf spec digest`, values equal) and the reviewer id matches `harness/reviewer-pool.json`
(`nan/deepseek-v4-flash`). I did not execute the archive — it has no dry-run and it moves the
folder — so the gate's acceptance is reasoned from its documented pre-flights (freshness by
contract digest, pool signature, promotion lines all answered), not observed.

### Recommended next steps

The contract set (`proposal.md`, `tasks.md`, `features.json`) is closed by this verdict — editing
any of them would invalidate it. Everything below is outside that set.

- **Finding 1 (code + tests, recommended before archiving):** make the v6 scan an exposure test
  rather than a reuse of the legacy-rule-shape test — e.g. treat a negated `-i`/`-s`/`-d` and an
  explicit any prefix (`::/0`, and for symmetry `0.0.0.0/0` in the v4 read) as unscoped for
  reporting — and add one named `TestIP6Exposure` case per shape. If you prefer to keep the AC3
  scope as written, file a bitácora ticket with the root cause instead and record the decision in
  `verification.md`.
- **Finding 2:** either let a v6 exposure set the step outcome to `manual` even when the v4 step is
  `unverified`/`failed`, or record in `verification.md` that AC3's "manual" is the converged-v4
  case only. Add the combined case to `TestIP6Exposure` either way.
- **Finding 3:** carry the tool's stderr into the v6 note the way `chainMissing` does, so an
  IPv6-disabled host says so.
- **Finding 4:** assert the check path's v6 call sequence in `TestIP6Exposure`, so the
  `withoutIP6Probe` filter is not the only thing standing between the tests and a misplaced probe.
- **Finding 5:** complete the AC1 citation in `verification.md` to name
  `TestFirewallStepsTailscaledModes/nodivert` for "setup writes nothing".
- Record each disposition (applied, ticketed, or declined with a reason) in `verification.md`; do
  not edit the contract set to absorb them.
