---
spec: "BUG-736-negated-portless-exposure"
verdict: "FAIL"
reviewed_sha: "b6a6c7b32c13b7f376a9899ac61cdafb37ada514"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: `BUG-736-negated-portless-exposure` — `git diff 46b921c1a5c914f517582599bbda05c30386d59b...HEAD` (14 first-parent commits, 617 insertions across `internal/host/`, `scripts/e2e/linux.sh`, the ADR/parity/lesson docs and the spec folder).
**Sources**: `specs/BUG-736-negated-portless-exposure/{proposal,tasks,verification}.md`, `features.json`, `review-request.json` (contract digests re-checked with `dotf spec digest` — they match), the diff, and `go test` / `golangci-lint` runs plus scratch probes (reverted) in this worktree at `b6a6c7b`.

**Reviewed at** `b6a6c7b`. `git rev-parse HEAD` = `b6a6c7b32c13b7f376a9899ac61cdafb37ada514`.

### What was verified by running it

- `go build ./...` — exit 0.
- `go test ./internal/host/ -run 'TestExposureShapes|TestWideAllowFix|TestReadIptablesUnscoped|TestIP6Exposure|TestSetupForLinux|TestCheckForLinux|TestIP6' -count=1` — `ok`.
- `go test -race ./internal/host/ -count=1` — `ok  ts-bridge/internal/host  1.276s`.
- `golangci-lint run ./internal/host/...` — `0 issues` (pinned 2.14.0 from mise).
- `gofmt -l .` — empty; `go vet ./internal/host/` — clean.
- Probes (scratch file, deleted afterwards; `git status` clean outside `specs/`): (a) `iptablesDecide` / `iptUnscopedRead` / `firewallCheckStep` / `ip6Exposed` on constructed `iptables -S` output; (b) the **production** path — `readLinuxFirewall` on a `recordingRunner` whose only simulated inputs are the command outputs, then `firewallCheckStep` and `firewallSetupFrom` — for the shape in F1. Outputs are quoted in the findings.

### Spec and task alignment

- Proposal, tasks and verification are complete and internally consistent: `iptExposes` replaces `iptPortCovers` in both reads, `iptScopedAllow` replaces `iptHasScope` in the allow read, `keepOpenFirst` is shared with #691's policy fix, ADR-017, `docs/design/host-client-parity.md` and `lesson-055` are amended in place (handoff evidence present).
- `iptablesDecide` is deliberately unchanged, as the proposal says; a negated port is listed as out of scope there.
- `tasks.md` ticks are backed by diff evidence for AC2/AC3 and by named tests for most of AC1 — the exception is the finding below: the AC1 claim is broader than any test.
- Contract digests match `review-request.json`; no `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags remain.
- The spec's own `verification.md` misattributes the v4 residual's mechanism (see F3).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Blocker | REAL | exposure read (v4), AC1 | A `! --dport 22` allow that is the **only** allowance is found by the scan but never surfaced: `iptUnscoped` returns it in `foreign`, yet `firewallCheckV4` / `firewallSetupFrom` reach `foreign` only through `legacyHeld`, which requires `state == rulePresent`, and `iptablesDecide` deliberately ignores a negated port. Result: `check` says **"No firewall rule found for port 3389"** (`changed`) and `setup` appends the scoped rule reporting `changed` — the wide rule is never named and the SSH caution never appears, while the port stays open off the tailnet. AC1 asserts this shape is "reported `manual` by `check` and `setup`, naming the rule". Reachable where nothing else decides the port — `tailscaled --netfilter-mode=off`, or an operator who removed the `tailscale0` accept — and the identical line **is** reported by the v6 scan, so the two reads disagree on one rule. Full-path probe (production `readLinuxFirewall` + `firewallCheckStep` / `firewallSetupFrom`, `iptables -S` output simulated, `ts-input` absent as with `--netfilter-mode=off`, no ufw, no `nft`, no `ip6tables`) on `-P INPUT DROP` + `-A INPUT -i lo -j ACCEPT` + conntrack ESTABLISHED + `-A INPUT -p tcp -m tcp ! --dport 22 -j ACCEPT`: `state=absent reason="" foreign=[-A INPUT -p tcp -m tcp ! --dport 22 -j ACCEPT] scoped=false`; `check: outcome=changed message="No firewall rule found for port 3389"`; `setup: outcome=changed message="Firewall rule added: TCP 3389 on tailscale0 (iptables)"`, its only mutation being `run iptables -A INPUT -i tailscale0 -p tcp --dport 3389 -j ACCEPT` — the wide rule is neither named nor deleted. Same line through `ip6Exposed` → reported, so the two reads disagree. The portless twin on the same host reads `manual` correctly (`legacyHeld=true`). | UNTESTED for this shape. Every step-level test puts the wide rule **beside** `iptScoped` (`TestCheckForLinux/iptables,_an_accept_for_every_port_but_SSH:_manual_(#736)`, `TestSetupForLinux/...(#736)`, and the e2e cases after the earlier tailscale0 steps) or is a portless rule `iptablesDecide` already counts. `TestExposureShapes` proves the predicate, not the reporting path. | code (surface the exposure when `len(read.foreign) > 0` regardless of `state`, without claiming the port is scoped) + tests (named check/setup regression for the only-allow shape) |
| Major | REAL | v4/v6 symmetry, scope read | A **negated non-loopback scope on a port-bearing rule** is reported by v6 and not by v4: `ip6Narrowed` reads a negated `-i/-s/-d` as absent and reports it; v4's `iptScopedAllow` treats any `!` that is not a port list or loopback scope as a scope. Host with the scoped rule plus `-A INPUT -p tcp --dport 3389 ! -s 100.64.0.0/10 -j ACCEPT` reads `ok` / `firewall_ok: true` on v4 while the port is reachable from every non-tailnet source. Pre-existing (old `iptHasScope` did the same), disclosed as a residual and stated to be tracked by #743 — but `verification.md` blames `iptPortlessCovers`, which is the mechanism only for the **portless** shape; for a port-bearing rule the mechanism is `iptScopedAllow`, so the residual description does not cover the shape it claims. | Code read of `iptScopedAllow`/`iptOpenNegation` vs `ip6Narrowed`; the v4 non-reporting is pinned by the pre-existing rows `TestReadIptablesUnscoped/{"a negated source does not admit the tailnet"}` and `{"/a negated tailnet interface ..."}`, the v6 reporting by `lesson-055`'s "a negated `-i`/`-s`/`-d` … narrows nothing and is" plus `TestIP6Exposure`. #743's scope could not be read (`gh` rate-limited) — UNVERIFIED. | `TestIP6Exposure` (v6 side) + `TestReadIptablesUnscoped` (v4 side) | code (align v4 with v6) or spec (name it in `proposal.md` Risks, not only `verification.md`) + a named failing test for the v4 shape |
| Minor | REAL | fix text | The SSH caution is attached only to a foreign rule that names **no** destination port, so a rule that also names SSH — `-A INPUT -p tcp -m multiport --dports 22,3389 -j ACCEPT` — is reported `manual` with the fix "scope … or remove it" and **no** caution, although removing it closes SSH under a DROP policy. Declared as a boundary by the proposal ("a rule with no destination port"), but the stated rationale ("removing such a rule can close SSH") applies here too. Pre-existing for port-bearing rules. | Probe: `check: outcome=manual message="… legacy unscoped rule: -A INPUT -p tcp -m multiport --dports 22,3389 -j ACCEPT"`, fix has no `keep loopback` clause; `TestWideAllowFix` pins the absence. | `TestWideAllowFix` (pins the absence — passes as written) | code (extend the caution to any foreign rule admitting the SSH port) or spec (soften "or remove it") |
| Minor | THEORETICAL | v4 scope read | `-A INPUT -p tcp -d 0.0.0.0/0 -j ACCEPT` is classified as scoped, so `check` answers "every allow is scoped to another interface or address" for a rule that narrows nothing — v6 has the explicit `::/0` exception in `ip6Narrowed`, v4 has no analogue. The step still reads `changed` and `firewall_ok` is false, and `iptables -S` normally omits a default destination, so reachability is unproven. | Probe: `state=present foreign=[] scoped=false` → `check: outcome=changed message="Firewall allows TCP 3389 (iptables), but not on tailscale0: every allow is scoped to another interface or address"`; `ip6Exposed("-A INPUT -d ::/0 -j ACCEPT")` reports. | `TestExposureShapes` has no `-d 0.0.0.0/0` row; UNTESTED for the v4 message | spec (declare the asymmetry) or code (mirror the `::/0` exception) — do not gate on it |
| Question | — | evidence | AC4/e2e is unverified here: no throwaway VM is available in this worktree and `make e2e-linux` was not run. `verification.md` reports `102 pass, 0 fail` at `f8ac37b`; `features.json` f3 points back at that line (circular). Also, every new e2e case runs beside a tailscale0 scoped rule, so the e2e structurally cannot catch F1. | Not reproducible in this review | `scripts/e2e/linux.sh` (the eight #736 cases) — UNVERIFIED | tests (add the only-allow shape; attach the command output) |
| Question | — | tracking | #743's existence and scope could not be checked: `gh issue view` returns "API rate limit already exceeded". F2's disposition rests on it. | `gh` output | — | spec (record the ticket title in `verification.md` so a reviewer can check it without the API) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | AC2/AC3 hold and the predicates are well covered; AC1 fails for the negated shape when it is the only allowance, and v4/v6 disagree on one rule. |
| Verification       | C | Unit-level claims reproduce (tests, race, lint), but AC1's claim exceeds the named tests and AC4's e2e evidence is a prose line I could not reproduce. |
| Scope              | A | Diff matches the proposal; docs, ADR-017, parity and lesson-055 amended in place; no creep. |
| Reliability        | B | Fail-closed on unmodelled narrowings and no error-path change; the decide/exposure split is what makes F1 report `changed` instead of `manual`. |
| Maintainability    | B | Small helpers, table-driven tests, comments explain the why; `iptPortlessToken` carries its baseline `gocyclo` waiver. |
| Handoff-readiness  | B | ADR, parity doc, lesson and verification updated; the residual's mechanism is misattributed and the ticket is uncheckable. |

### Verdict

**FAIL** — one REAL Blocker (F1: an AC1 shape is not reported `manual` when it is the only allowance, and the operator is told the opposite) and one REAL Major (F2: v4 vs v6 asymmetry on a negated non-loopback scope), plus minors. The rubric alone would read PASS WITH GAPS (one C, no D); the severity axis escalates to FAIL.

### Recommended next steps

F1 and F2 are contract-set-adjacent, so the next round is a fix-and-re-review, not a disposition-and-archive:

1. **Code** — surface the exposure for v4 even when `state == ruleAbsent`: report `len(read.foreign) > 0` as `manual` (with `foreignFix`, so the SSH caution lands) from `firewallCheckV4` and from `firewallSetupFrom` before the "add the scoped rule" branch, without claiming the port is scoped. Do **not** change `iptablesDecide` (the proposal keeps it unchanged).
2. **Tests** — a named red-green regression for the only-allow shape: `TestCheckForLinux/iptables,_an_accept_for_every_port_but_SSH_as_the_only_rule:_manual_(#736)` and a setup twin asserting the scoped rule is added, the wide rule is **named** and nothing is deleted. Prove red by reverting step 1.
3. **Code or spec (F2)** — either align v4 with `ip6Narrowed` for a negated non-loopback scope, or move the residual into `proposal.md` Risks with its real mechanism (`iptScopedAllow` for port-bearing rules, `iptPortlessCovers` for portless) and a named test pinning the current, accepted behaviour. `verification.md`'s "because `iptPortlessCovers` …" sentence should say both.
4. **Tests (F3)** — extend the caution to a foreign rule that names the SSH port among others, or make the fix text stop offering "remove it" for it.
5. **Evidence (AC4)** — re-run `make e2e-linux` with an only-allow case added and paste the pass/fail counts and the case names into `verification.md`.
6. **Tracking** — record #743's title (or file the ticket) so the v4 residual is checkable offline.

Contract-set edits (`proposal.md`, `tasks.md`, `features.json`) are appropriate here only as part of the fix + re-review; they invalidate this review by design. `verification.md` edits (items 3–6) are outside the staleness check and can be made freely.
