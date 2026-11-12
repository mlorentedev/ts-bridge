---
spec: "SEC-631-scoped-firewall-rules"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "94cf5d735caad04acb948701e8f0e8f40a714469"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-10"
---

## Adversarial review

**Scope**: SEC-631-scoped-firewall-rules — `git diff 9ec0c909fc8d194839b3f7bebade48ba366e29cb...HEAD`
(18 first-parent commits; this spec's are `80108b2` ADR-017 Amendment B, `6d53dd6` iptables,
`55cfd68` Windows, `af7a330`/`27ccaf9`/`f08036a` review-round fixes, with the spec and e2e docs
alongside. The rest of the range is other specs' work that a rebase onto master pulled in; judged
only where it touches this change.)
**Sources**: `specs/SEC-631-scoped-firewall-rules/{proposal,tasks,verification,features}.md`;
`internal/host/{linux_steps,windows_steps,linux_steps_test,windows_steps_test}.go`;
`scripts/e2e/{linux.sh,windows.ps1}`; `docs/adr/adr-017-host-desired-state.md` (Amendment B),
`docs/runbooks/guide-rdp-host-setup.md`, `docs/design/host-client-parity.md`.

### Evidence I produced this run

| Command | Result |
|---|---|
| `go test -race ./internal/host/` | `ok ts-bridge/internal/host 1.358s` |
| `go build ./...` | exit 0 |
| `gofmt -l internal/host/` | no output |
| `GOOS=windows go vet ./internal/host/` | clean |
| `dotf spec digest SEC-631-scoped-firewall-rules` | byte-identical to the four `contract_digests` in `review-request.json` |
| `grep AGENT-DRAFT\|AGENT-SUGGESTION` in the spec folder | none in the contract files (only quoted in the older reviews' prose) |
| `git diff --diff-filter=D --name-only` | no test file deleted; 8 test funcs added, none removed |
| e2e (`make e2e-linux`, `make e2e-windows`) | **not re-run** (throwaway VMs, out of this run's budget); verification.md's logs are the only evidence, reasoned about below |

### Spec and task alignment

- **AC1** — Amendment B exists in `adr-017-host-desired-state.md` with the hybrid scope, the
  two-tier replacement, the Windows group decision, the rejected alternatives and the
  `PASS`-relevant residuals. The §5 table rows were rewritten to match. Good.
- **AC2/AC3/AC4** — the append/allow helpers are the single write path (`iptablesAppend`,
  `ufwAllowArgs`, used by `addScopedRule`, `ufwInactiveReplace` and the fix strings), the legacy
  detectors (`iptLegacyCopies`, `ufwUnscopedRead`) sit behind named tests, and `check`'s
  `firewall_ok` now also requires `!legacyHeld && !offTailnetOnly`. The scoped rule is written
  before the delete in both backends (`legacySetupStep`: add when `!read.scoped`, then
  `legacyDeletes`), and on the inactive-ufw path (`ufwInactiveReplace`). Matches the proposal's
  two tiers, including the deliberate refusals (`foreign` reported, never deleted).
- **AC5** — `-RemoteAddress 100.64.0.0/10,fd7a:115c:a1e0::/48`, `remoteIsTailnet` normalises the
  dotted IPv4 mask and folds IPv6 case, `Enable-NetFirewallRule` is gone, `$old` is captured
  before the create and removed by that identity rather than by display name, and the group is
  reported `manual` with the disable command and never disabled. `firewall_ok` flows from the
  step's `ok` through `windowsFirewallCheckStep` → `svc.FirewallOK` → `result.FirewallOK`
  (`step.go:160-165`), so the group really does clear the key.
- **AC6** — both scripts carry the #631 cases; see finding 3 for the one place the claim is
  narrower than the AC wording.
- **AC7** — `offTailnetOnly` + `TestOffTailnetOnlyAllow` (iptables source/interface, ufw
  source/interface) and the e2e cases on both backends; `iptAcceptOffTailnet` also treats a
  negated scope as off the tailnet, which is the right fail-closed reading.
- Round-1 and round-2 dispositions in `verification.md` are consistent with the code I read: the
  inactive-ufw fix is in `ufwInactiveState`/`ufwInactiveReplace`, the garbled-count fix is in
  `builtinGroupStep` (parse error → `unverified`), and the negated-scope fix is `iptNegated`.
- One documented fork to check against code: "an allow scoped to an address counts as off the
  tailnet even when the address is `100.64.0.0/10`". `iptAcceptOffTailnet` returns true for **any**
  `-s`/`-d`, including on a `-i tailscale0` rule — i.e. an operator's
  `-i tailscale0 -s 100.64.0.0/10 …` is treated as off-tailnet and setup adds a second allow
  beside it. That is exactly what the proposal records; it converges on the second run, so it is
  a duplicate-rule annoyance, not a defect.
- `features.json` names the tests that exist (`TestIptablesPrintedForm`,
  `TestReadIptablesUnscoped`, `TestSetupForLinux`, `TestCheckForLinux`, `TestUFWAllowArgs`,
  `TestReadUFWUnscoped`, `TestWindowsFirewallRecreateScoped`, `TestWindowsFirewallRule`,
  `TestWindowsFirewallRecreateOrder`, `TestWindowsBuiltinGroup`, `TestOffTailnetOnlyAllow`).
  `state`/`evidence` are still `pending` for all 16; those fields are harness-owned and outside
  the digest, so this is bookkeeping for the tooling, not a contract drift.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | THEORETICAL | Windows reporting | `psReadRDGroup` selects the built-in group by its **localized display name** (`-DisplayGroup 'Remote Desktop'`). On a non-English Windows the query resolves to no rule objects, so `exposed=0` and `builtinGroupStep` returns the step unchanged: `check` and `setup` report the firewall ok while the group's any-address RDP rules are enabled — the exact exposure AC5 exists to report. The ADR notes the interface alias is localised but not that the same weakness applies to the group name. | Code read: `windows_steps.go` `psReadRDGroup`, `builtinGroupStep` (`exposed == 0` → `return step, true`). Could not run a localized guest here. | UNTESTED — `TestWindowsBuiltinGroup`'s "none enabled" row is the same unqualified `exposed=0` answer read as clean, and `windows.ps1`'s `RdGroupOn` uses the English name too, so an English image cannot catch it. | code (query the group by its invariant identifier via `-Group`, or report `unverified` when the query returns no rule objects at all) + tests |
| Question / assumption | REAL | Linux reporting residual | On a host with `iptables -P INPUT ACCEPT` (the distro default where ufw is absent or inactive) or a portless `-A INPUT -p tcp -j ACCEPT`, the port remains reachable off-tailnet after this change and `check` still reports `firewall_ok: true`. This is the original exposure, not a regression. | `iptablesDecide` skips every line whose first field is not `-A`, so `-P INPUT ACCEPT` never decides and nothing else reads the policy; a portless accept is `ipt.scoped` by design (`offTailnetOnly` false, `legacyHeld` false). ADR Amendment B states it: "under `iptables -P INPUT ACCEPT` the scope restricts nothing and `check` does not say so (#691)", and the proposal lists the portless accept as an operator-posture fork. | UNTESTED — no row covers a permissive policy or a portless accept as "converged while open". | code/spec follow-up; the policy half is already #691, the portless half has no ticket |
| Minor | REAL | Verification | AC6 says the e2e harness covers both OSes "each run on a real VM". The Windows e2e last ran at `27ccaf9`/`310bfc6`; `f08036a`'s Windows half (8 lines) was not re-run there, only covered by unit tests. The claim is honest — verification.md states it and `git show --stat f08036a` confirms the Windows diff is message-only — but the AC wording reads wider than the evidence. | `git show --stat f08036a`: `windows_steps.go` 8 lines (the unparsed-count branch), `windows_steps_test.go` 9. | `TestWindowsBuiltinGroup` ("a garbled count is unverified", "a non-numeric count is unverified") | verification.md (state the message-only scope beside AC6) — no code change |

Nothing else reached finding status after trying to break it: a failed add deletes nothing and a
failed delete names the command; create-before-remove and its terminating error are asserted
(`TestWindowsFirewallRecreateOrder`); a ufw-scoped allow is never deleted; a deny that decides the
port short-circuits both commands (fails closed); `remoteIsTailnet` compares a normalised set and
rejects `Any`; `legacyDeletes` only ever emits the exact pre-#631 forms.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | Every AC is met and the negative paths are well covered; one THEORETICAL Windows reporting gap and one owner-accepted permissive-policy residual. |
| Verification       | A | Reproducible unit commands, named e2e logs, mutant lists, and red-first evidence; the one gap (localized Windows) is not reachable from this environment. |
| Scope              | B | The spec's own commits match the proposal and tasks; the range also carries other specs' work from rebasing onto master, and the Windows e2e was not rerun after a message-only fix (stated). |
| Reliability        | A | Add-before-delete on both backends, terminating errors, identity-based removal, and idempotency pinned by a second-run e2e case. |
| Maintainability    | B | New helpers are small and carry why-comments; the file retains four `//nolint:gocyclo // baseline #629` annotations from the complexity baseline. |
| Handoff-readiness  | B | ADR, runbooks, parity doc and verification are current; lesson capture is explicitly declined with a reason rather than deferred silently. |

### Verdict

PASS WITH GAPS — no Blocker, no REAL Major. One open Major is THEORETICAL and tracked below; the
only REAL gap is an accepted residual that the ADR and proposal record, half of it as #691.

### Recommended next steps

Contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed**: this verdict describes the
current digests, so an edit there invalidates the review. Put these in `verification.md` instead,
or carry them into a follow-up ticket.

1. **Disposition finding 1** (code + tests, follow-up, not blocking this archive): either query
   the built-in group by its invariant identifier instead of the localized display name, or treat
   an `exposed=0` answer whose query returned no rule objects at all as `unverified`. A regression
   row in `TestWindowsBuiltinGroup` for "the display name resolves to nothing" would name it. This
   is the same fail-closed principle round 1 applied to the garbled count.
2. **Record finding 2** as a tracked residual: the portless `-A INPUT -p tcp -j ACCEPT` case has
   no ticket (the `-P INPUT ACCEPT` case is #691). A one-line note in `docs/troubleshooting/` or
   the RDP runbook saying `check` reports the rule it expects, not the host's policy, would stop
   the `firewall_ok: true` from being read as "the LAN cannot reach the port".
3. **Tighten AC6's wording** in `verification.md` (not `proposal.md`): the Windows e2e last ran at
   `27ccaf9`; `f08036a`'s Windows half is message-only and unit-pinned.
4. Optional: `features.json` states are all `pending`; the selector guard will want them flipped
   when the evidence is registered.
