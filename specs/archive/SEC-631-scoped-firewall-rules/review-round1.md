---
spec: "SEC-631-scoped-firewall-rules"
verdict: "FAIL"
reviewed_sha: "55cfd68ba8ef0f91d8ecfc12669895e1498e216c"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-09"
---

## Adversarial review

**Scope**: SEC-631-scoped-firewall-rules — `git diff 9ec0c909fc8d194839b3f7bebade48ba366e29cb...HEAD`
(12 first-parent commits, of which 4 are this spec's: `80108b2` ADR-017 Amendment B, `605e5d2`
iptables, `6d53dd6` ufw, `55cfd68` Windows). The rest arrived through merges from master
(#674/#676/#675/#677/#690/#698/#696/#681, SEC-391) and were judged only where they interact.
**Sources**: `specs/SEC-631-scoped-firewall-rules/{proposal,tasks,verification,features.json}`,
`internal/host/{linux_steps.go,windows_steps.go}` + their tests, `scripts/e2e/{linux.sh,windows.ps1}`,
`docs/adr/adr-017-host-desired-state.md`; `contract_digests` in `review-request.json` match
`dotf spec digest SEC-631-scoped-firewall-rules` (identical four hashes).

### What I ran (fresh, this session, at `55cfd68`)

| Command | Result |
|---|---|
| `go test -race ./...` | exit 0, no failures |
| `golangci-lint run ./...` (pinned 2.14.0, host/darwin) | `0 issues.` |
| `gofmt -l .` | empty |
| `GOOS=windows go vet ./...`, `GOOS=linux go vet ./...` | clean |
| `bash scripts/check-features-selectors.sh` | `OK (50 go test -run selectors)` — every `features.json` selector resolves |
| `bash scripts/check-features-shape.sh` | `OK (39 features.json files)` |
| `features.json` f1 verification (grep on ADR-017) | exit 0 |
| Mutation: drop `-i tailscale0` from `iptablesAppend` | killed — `TestIptablesPrintedForm` fails on both the printed form and the append string |
| `make e2e-linux` / `make e2e-windows` | **not run** — requires `dotf vm` guests; AC6 is UNVERIFIED here |

Probes were run in a throwaway copy (`/tmp/probe631`, `tar` of the worktree, no `.git`); the repo
tree is clean apart from this file (`git status --porcelain` shows no tracked change).

### Spec and task alignment

- AC1 (ADR-017 Amendment B): present, and f1's command passes. ✔ verified.
- AC2/AC4 (iptables): the scoped append, the printed form, the two replacement tiers, the failed-add
  and failed-delete paths all exist with named tests (`TestIptablesPrintedForm`,
  `TestReadIptablesUnscoped`, `TestSetupForLinux`, `TestCheckForLinux`). ✔ verified, with one gap
  (finding 1).
- AC3 (ufw): one write helper (`ufwAllowArgs`), pinned by `TestUFWAllowArgs`; legacy/foreign split by
  `TestReadUFWUnscoped`. ✔ verified.
- AC4 negative path: foreign rules are reported `manual` and left in place — pinned by
  `TestSetupForLinux`/`TestCheckForLinux` and by the e2e cases (script read, not run).
- AC5 (Windows): scoped recreate, set comparison of `RemoteAddress`, create-before-remove with
  terminating errors, built-in group reported and never disabled —
  `TestWindowsFirewallRecreateScoped`, `TestWindowsFirewallRule`, `TestWindowsFirewallRecreateOrder`,
  `TestWindowsBuiltinGroup`. ✔ verified as unit-level behaviour, with the premise doubt in finding 2.
- AC6 (e2e both OSes): both scripts carry the cases; the Windows run is **UNVERIFIED** here.
- The two unchecked "Closing (every PR)" boxes in `tasks.md` (go test / lint / gosec / verification
  filled) contradict `verification.md`, which claims all of it — finding 3.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | correctness / AC2 | A host whose **only** allow for the port is scoped to something other than the tailnet (a LAN source, another interface) makes `check` return `ok` with `firewall_ok: true` and `setup` return `ok`, while setup never writes the `-i tailscale0` rule — so the port stays unreachable over the mesh and the tool reports convergence. `scoped` is computed correctly (`false`) by `iptAcceptOffTailnet`/`ufwAllowOffTailnet`, but is only consulted when a legacy/copy rule is also present (`legacyHeld` requires `ufwLegacy \|\| copies>0 \|\| foreign>0`), never on its own. | Reproduced against a copy of the tree: iptables host, `-P INPUT DROP` + `-A INPUT -s 10.0.0.0/8 -p tcp -m tcp --dport 3389 -j ACCEPT` → `state=2 copies=0 foreign=[] scoped=false`, `check outcome="ok" msg="Firewall allows port 3389"`, `setup outcome="ok" msg="Firewall allows TCP 3389 (iptables)"`, `calls=[lookpath ufw lookpath iptables output iptables -S INPUT]` (no append). ufw host, `ufw show added` = `ufw allow in on eth1 to any port 3389 proto tcp` → `scoped=false ufwScoped=false`, `check outcome="ok"`, `setup outcome="ok" msg="Firewall allows TCP 3389 (ufw)"`, no `ufw allow …` call. The allow lines are the ones the fixtures in `TestReadIptablesUnscoped` / `TestReadUFWUnscoped` call "a source address does not admit the tailnet" / "another interface does not admit the tailnet"; my probe only tightens the iptables host to `-P INPUT DROP`, so the port really is unreachable from the tailnet rather than merely unaddressed. | UNTESTED — no test asserts the step outcome for the off-tailnet-only state; the two fixtures only pin `read.scoped` | code + tests (a `TestSetupForLinux`/`TestCheckForLinux` case: `changed` and an append; and an e2e case), or a proposal entry listing it as accepted |
| Major | THEORETICAL | correctness (Windows) | The built-in "Remote Desktop" group is reported `manual` whenever it is **enabled**, and the proposal defends that with "on an upgraded host the group is usually enabled *by setup*". Windows ships those rules enabled (Profile = Domain/Private) by default, so on a *pristine* host the report fires too: `check` exits non-ok and `host setup` never converges until the operator disables Microsoft's own group. The e2e cannot see this: it calls `Enable-NetFirewallRule` itself and only asserts the subsequent `firewall_ok` after its own `Disable-NetFirewallRule`, so a default-enabled guest passes identically. | Code: `psReadRDGroup` selects `-Enabled True … RemoteAddress -eq 'Any'`; `builtinGroupStep` turns any non-zero count into `OutcomeManual`. Script: `scripts/e2e/windows.ps1` "built-in … enabled" / "disabled by the operator" cases, both self-driven. Not observed on a host (no Windows guest here). | UNTESTED — no case reads the group state *before* touching it, so the default premise is unverified | spec (proposal premise + an accepted-impact note) + tests (a Windows e2e case asserting convergence on an untouched `check`) |
| Minor | REAL | spec artifacts | The four spec files disagree about whether the Windows e2e ran. `proposal.md` Risks: "the Windows guest is shared with another session, so PR 4 says its e2e is pending"; `features.json` f12: "… (pending: the Windows guest is shared)"; `tasks.md` PR 4: "cases written and run: `make e2e-windows` 50 pass, 0 fail at 310bfc6"; `verification.md`: "Windows e2e run (see PR 4 below)" with 50/0. (`310bfc6` and `66f28d6` are real commits, "Merge origin/master into fix/issue-631-windows-scope" and the ufw off-tailnet fix.) | file read | n/a (artifact) | spec + verification.md |
| Minor | REAL | spec artifacts / handoff | `tasks.md`'s "Closing (every PR)" items are still `[ ]` (`go test -race ./...`, `golangci-lint` host+linux+windows, `gosec ./...`, `verification.md` filled) although `verification.md` asserts them; `verification.md` has no "Promotion candidates" section, which `dotf spec archive`'s third pre-flight reads (whether its absence is accepted is UNVERIFIED — I did not run the archive, it moves the folder). | `tasks.md` lines under "## Closing (every PR)"; `grep -n "Promotion candidates" specs/SEC-631-scoped-firewall-rules/*.md` → none | n/a (artifact) | spec (tasks.md, contract set) + verification.md |
| Minor | SPECULATIVE | resilience (Windows) | An unparsable/empty `exposed=` value reads as `0` (`exposed, _ := strconv.Atoi(...)`), so a garbled read is silently treated as "group clean" rather than unverified — the opposite of the read-failure branch two lines above. | `builtinGroupStep` in `internal/host/windows_steps.go` | UNTESTED | code + tests |
| Minor | SPECULATIVE | correctness (iptables edge) | `iptUnscoped` is computed from the raw INPUT text, while the "scoped" walk runs over the *inlined* ts-input chain; an unscoped port ACCEPT living in `ts-input` therefore counts as the scoped rule and is neither reported nor replaced. tailscaled does not write such a rule (its accept is `-i tailscale0`), so no repro. | `iptUnscopedRead` / `iptInlineChain`; `tsInputDefault` fixture | UNTESTED | code + tests (or a comment recording the assumption) |

No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags remain in the spec folder.

**Not a finding, recorded for the next runner:** the very first `go test ./internal/host/...` of this
session reported two failures in `TestWindowsRDPDenyState` / `TestWindowsReadsReachTheirParsers`
(`effective() = false, false; want true, true`). It did not reproduce in 26 further runs
(`-count=1`, 25 in a loop), nor in the full `-race` run; I could not attribute it to the change and
treat it as an artifact of the checkout, not a defect. UNVERIFIED either way.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | Every enumerated AC path is implemented and tested, but a state the spec's own helpers classify (`scoped=false`, an off-tailnet-only allow) yields a false `ok` on both backends (finding 1). |
| Verification       | B | Reproducible named tests, mutant kills and e2e counts are recorded, and `go test -race`, lint, gofmt and vet reproduce clean here; AC6 needs `dotf vm` guests and could not be reproduced in this session. |
| Scope              | B | The branch's four PRs match `tasks.md`; the range also carries merged master work (SEC-391, HOST-008, QA-607) that this review judged only where it interacts. |
| Reliability        | B | Failed add deletes nothing, failed delete names the command, Windows create precedes remove with `$ErrorActionPreference='Stop'`, the group is never disabled — but "converged" is asserted for a host the mesh cannot reach (finding 1) and one Windows read failure reads as clean (finding 5). |
| Maintainability    | B | New code passes the raised `funlen`/`gocyclo`/`nolintlint` gates; helpers are small and single-purpose; several functions carry `//nolint:gocyclo // baseline #629`, so the complexity picture in this package is baselined rather than reduced. |
| Handoff-readiness  | B | ADR amendment, `docs/design/host-client-parity.md` and the runbooks are updated and implementation decisions are written down; the Windows-e2e status is contradictory across four files (finding 3) and the closing checklist is unticked (finding 4). |

### Verdict
FAIL

One **REAL Major** (finding 1) — `check`/`setup` report a host converged when the tailnet cannot
reach the port — is reproduced, is UNTESTED, and is in the contract's own terms (AC2/AC4). The
rubric's C in Correctness is a second, independent path to the same verdict.

### Recommended next steps

All of these are for the implementer to disposition in `verification.md` (applied / ticketed /
declined with a reason) or to carry in the re-review round; on a FAIL the contract-set edits happen
**now**, and the archive waits for the next review.

1. **Finding 1 (code + tests, contract set too).** Make the scoped rule setup's own invariant: when
   `read.state == rulePresent` and `read.scoped` is false, append/replace as `legacyHeld` does today
   (`-i tailscale0` on iptables, `ufw allow in on tailscale0 …`), report `changed`, and say why in the
   step message so an operator with a LAN-scoped rule understands what was added. Add a
   `TestSetupForLinux`/`TestCheckForLinux` case using the existing
   `-A INPUT -s 10.0.0.0/8 …` and `ufw allow in on eth1 …` fixtures, plus one e2e case. If the owner
   prefers the current behaviour, record it in `proposal.md`'s "Decision forks" instead — that is a
   contract edit and needs this review redone either way.
2. **Finding 2 (spec + Windows e2e).** Verify the group's state on a *pristine* guest before any
   `Enable-NetFirewallRule`, and either re-word the proposal's "usually enabled by setup" premise or
   add the accepted-impact note (with the disable command) to the Windows runbook. Add the e2e case
   that asserts the first `check` on an untouched guest, so the default state is part of the record.
3. **Findings 3–4 (spec artifacts).** Reconcile the Windows-e2e status to one statement across
   `proposal.md` Risks, `tasks.md` PR 4, `verification.md` and `features.json` f12; tick or re-word
   the "Closing (every PR)" items; add a "Promotion candidates" section with `no: <reason>` lines
   (the ADR-017 amendment and the choice of `tailscale0` as a fixed constant are the obvious
   candidates) before archiving.
4. **Finding 5 (code + tests).** Treat an unparsable `exposed=` as `OutcomeUnverified` (the same
   branch as a failed read) and pin it with a `TestWindowsBuiltinGroup` case.
5. **Finding 6 (code comment or test).** Either exclude ts-input's inlined lines from the "scoped"
   walk or record why an unscoped accept there is treated as the scoped rule.

Archive is **not** advisable in the current state: the review is FAIL and, separately, the contract
set still needs the finding-1 decision.
