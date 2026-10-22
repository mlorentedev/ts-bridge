---
spec: "BUG-035-iptables-scoped-allow"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "8733c9b679c3161e613045beba40aef0621ceb4e"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-22"
---

## Adversarial review

**Scope**: BUG-035-iptables-scoped-allow — round 2, the whole change (`8fd5fe8...HEAD`, two commits: `ef191e4` the
`iptables -C` → `iptables -S INPUT` coverage read plus the ufw no-destination-port allow, `8733c9b` the round-1
test findings).
**Sources**: `specs/BUG-035-iptables-scoped-allow/{proposal,tasks,verification}.md` + `features.json`;
`git diff 8fd5fe855c4411094534945316c64edf1b6c199d...HEAD` and `git log` over the same range; round-1 review
(`specs/BUG-035-iptables-scoped-allow/review-round1.md`).

Contract freshness: `dotf spec digest BUG-035-iptables-scoped-allow` returns exactly the three
`contract_digests` in `review-request.json` (features.json, proposal.md, tasks.md all match), so this verdict
describes the current contract. No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain in any spec file.

### Spec and task alignment

- AC1 → `iptablesCovers` + `TestIptablesCovers`, plus the two new round-2 check-side assertions
  (`TestCheckForLinux/iptables, scoped allow`, `TestSetupForLinux/iptables, converged`). Met.
- AC2 → `TestSetupForLinux/iptables, rule missing` (exactly one `run iptables -A ...`, `changed`),
  `TestIptablesRule/absent behind a ts-input jump`. The "second run reports `ok`" clause is not a named unit
  test: it is proven by composition (`TestIptablesCovers/exact` covers the appended form, so the next read is
  `rulePresent`) and by the container run in `verification.md`, which I could not reproduce here → UNVERIFIED,
  not contradicted.
- AC3 → the 22-case `TestIptablesCovers` table; every negative I re-mutated turned red. Met.
- AC4 → the `destPortIndex(f) < 0` branch + `TestUFWAllows` (source-only, interface-only, source-only+tcp
  present; udp, app profile, source-port-only absent). Met.
- AC5 → `readLinuxFirewall` uses `Output(iptables, "-S", "INPUT")`; `iptablesAppend` is the only mutating argv;
  `TestIptablesRule/permission`, `/no exit status`, `TestSetupForLinux/iptables -S fails` (`failed`, no `-A`),
  `TestCheckForLinux/iptables -S fails` (`unverified`). The exact call lists in both setup and check cases
  assert that no other `run` happens. Met.
- `tasks.md`: every box is `[x]` except "PR opened", which is this review window (PR #482 is open as a
  **draft**), not a defect.
- `features.json` is honest: five entries, all `state: pending`, all `evidence` empty (the harness owns the
  terminal state), each `verification` targeting the named tests I ran.
- Round-1 dispositions are real, not verbal: `#483` exists and is open with the blanket-ACCEPT body, and #480/
  #481 cover the duplicate-rule and deny-shadowing items. The round-1 comment fix is **incomplete** — see
  finding 2.

Verification I ran fresh, in this session, on `8733c9b`:

| Command / mutation | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go test -race -count=1 ./...` | 13 packages ok, 0 failures |
| `GOOS=linux\|windows\|darwin go vet ./...` | clean, all three |
| `mise exec -- golangci-lint run` | `0 issues.`, exit 0 |
| `dotf spec digest` vs `review-request.json` | all three contract digests match |
| mutation: comment strip removed (`strings.Fields(iptComment.ReplaceAllString(line, ""))` → `strings.Fields(line)`) | red — `TestIptablesCovers/comment_holding_-j_ACCEPT` (round-1 finding 1 now has teeth) |
| mutation: `!`-precedes-flag check removed | red — `TestIptablesCovers/negated_port`, `/negated_multiport` |
| mutation: `-p tcp` requirement removed | red — `TestIptablesCovers/udp` |
| mutation: `--ports` dropped from the flag list | red — `TestIptablesCovers/source_or_destination_ports` |
| mutation: `-j ACCEPT` requirement removed | red — `TestIptablesCovers/drop` |
| mutation: ufw no-destination-port branch reverted to `return false` | red — `TestUFWAllows/source_only`, `/interface_only`, `/source_only,_tcp` |
| mutation: `-S` error collapsed into `ruleAbsent` | red — `TestIptablesRule/permission`, `/no_exit_status`, `TestSetupForLinux/iptables_-S_fails`, `TestCheckForLinux/iptables_-S_fails` |

Probes (throwaway test file, deleted afterwards; tree left clean apart from this review):

| Input | `iptablesCovers`/`ufwAllows` (port 3389) |
|---|---|
| `-A INPUT -i tailscale0 -j ACCEPT` / `-A INPUT -s 100.64.0.0/10 -j ACCEPT` / `-A INPUT -p tcp -j ACCEPT` | `false` (finding 1, tracked #483) |
| `-A INPUT -d 192.168.1.5 -p tcp -m tcp --dport 3389 -j ACCEPT` | `true` (finding 3) |
| `ufw allow in on tailscale0 to 192.168.1.5` | `true` (finding 3) |
| `-A INPUT -p tcp -m tcp --dport 22 -m tcp --dport 3389 -j ACCEPT` | `false` (finding 4) |
| `-A INPUT -p tcp -m tcp --dport 3389 -g ts-input` | `false` (correct: goto is not ACCEPT) |
| `-A INPUT -p tcp -m limit --limit 1/hour -m tcp --dport 3389 -j ACCEPT` | `true` (finding 4) |
| `ufw route allow in on tailscale0`, `ufw allow in on tailscale0 app OpenSSH`, `ufw allow from 100.64.0.0/10 proto any` | `false` (correct) |

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | THEORETICAL | iptables coverage vs ufw parity | A blanket ACCEPT with no destination port (`-i tailscale0 -j ACCEPT`, `-s 100.64.0.0/10 -j ACCEPT`, `-p tcp -j ACCEPT`) reads **absent**, so `host setup` still appends the unrestricted rule on the most common hardened Tailscale shape, while the ufw equivalents read **present**. Declared in proposal "Not covering", so not an AC violation — an unclosed gap in the headline ("both backends answer one question the same way"). Carried, not new: it is already ticketed. | Probe this session: both → `false`/`true` respectively; harm chain pinned by `TestSetupForLinux/iptables, rule missing`; ticket `#483` is open with the same evidence and root cause (`-m conntrack` exclusion over-generalised). | UNTESTED (no case on either backend) — exactly what `#483` asks for ("build the test table from verbatim `iptables -S INPUT` output") | code + tests, in the `#483` follow-up (needs a contract decision; do **not** edit `proposal.md` under this review) |
| Minor | REAL | maintainability (stale comment) | `internal/host/step.go:73` still documents the old read: "the Linux iptables read is a read-only `iptables -C` through Run". The read is now `-S INPUT` through `Output`, so the sentence is wrong in the one place that describes the read half's contract, and `verification.md` records round-1's comment finding as "applied" when only the two `linux_steps_test.go` comments were fixed. | `grep -rn "iptables -C" --include=*.go .` → only `internal/host/step.go:73` (tests and lesson-042 are fixed/historical). | UNTESTED (comment-only) | code (`checkFor` doc comment); free, outside the contract set |
| Minor | THEORETICAL | iptables/ufw parser, message accuracy | A destination-qualified allow (`-d 192.168.1.5 … --dport 3389`; `ufw allow in on tailscale0 to 192.168.1.5`) reads present, so `check` reports "Firewall allows TCP 3389" for a rule that selects a different address on the host and may not admit tailnet traffic. Not a widening (setup appends nothing), and `-i`/`-s` are declared the operator's scope — `-d` is the one qualifier that is *not* the same kind of scope. Same class on both backends, so parity holds. | Probes above: both → `true` for a destination-scoped rule. | UNTESTED | surface only — fold into `#481` if it recurs |
| Question | SPECULATIVE | iptables/ufw parser | `iptFlag` takes the **first** occurrence of a flag, so a legal-but-unusual rule repeating `--dport` (`--dport 22 -m tcp --dport 3389`) reads absent; a rate-limited ACCEPT (`-m limit --limit 1/hour … --dport 3389 … -j ACCEPT`) reads present even though it throttles the connection. `iptables-save` normalises `-p 6` to `-p tcp`, so the protocol-number variant is not a risk. Each mis-read leads to one extra narrow rule (absent) or no new rule (present), never to a widening beyond #479. | Probes above. No `iptables -S` output I could produce by hand selects both forms, and I cannot run the container here. | UNTESTED | — (surface only; do not gate) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | All five ACs verified by named tests I ran; the 22-case negative table is broad and every guard I mutated (comment strip, negation, protocol, `--ports`, target, ufw no-dest branch) turned red; no observed defect. Residual destination-scoped false positive is theoretical and declared. |
| Verification       | B | Re-runnable commands with real outputs for build/race/vet/lint/digest, and red-green proofs for every claimed mechanism; the privileged `ubuntu:24.04` run behind AC2's second-run clause and the ufw 0.36.2 measurements are not reproducible in this run — UNVERIFIED, not contradicted. |
| Scope              | A | Diff is the proposal plus the lesson/parity rows `tasks.md` names; no creep, no dependency, no unrelated file. |
| Reliability        | A | Unknown reads stay `unverified` (check) / `failed` (setup) with no mutation — mutation 7 proves both are pinned; append only on `ruleAbsent`, so a re-run converges. |
| Maintainability    | B | Functions are short (CC well under 10), names say what they do, both helpers carry why-comments; but one comment still describes the deleted `-C` read. |
| Handoff-readiness  | A | Lesson 043 records the widening and the measured `-S` syntax and is indexed; parity doc row updated; `verification.md` carries round-1 dispositions and answers every promotion line. |

Rubric alone is PASS (no C, no D). The open **Major THEORETICAL** (finding 1) is tracked as #483, which under
`severity × reality` is a tracked gap, not a blocker → **PASS WITH GAPS**.

### Verdict

PASS WITH GAPS

### Recommended next steps

The contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed** at this verdict — nothing below
requires a contract edit, and editing one would invalidate this review. Route the items through
`verification.md` (outside the staleness check) or the existing tickets.

- **code (comment-only, free)** — `internal/host/step.go:73`: replace "a read-only `iptables -C` through Run"
  with the current read (`iptables -S INPUT` through `Output`). Then record in `verification.md` that round-1's
  comment finding is fully applied, since it currently claims it is.
- **`#483` (already open, not a new ticket)** — the blanket-ACCEPT gap (finding 1). No action under this spec
  beyond keeping the disposition; the fix needs a contract decision on how to tell a blanket ACCEPT from a
  `-m conntrack` rule.
- **surface only** — destination-qualified allows (finding 3) and the repeated-`--dport` / `-m limit` shapes
  (finding 4): no gate, no ticket unless `#481` work reaches them.
- **PR** — #482 is still a draft; mark it ready (`gh pr ready 482`) once the comment fix lands so the human
  review reads the finished change, and run `dotf pr triage-queue` before calling the PR work complete.
- **UNVERIFIED, not a finding** — the privileged container end-to-end run and the ufw 0.36.2 measurements in
  `verification.md` were outside this run's budget; every unit-level claim they back held under my own runs.

`dotf spec archive` is advisable: the REAL Major that blocked round 1 is fixed and its regression test
demonstrably fails without the fix. The two open items are one comment and one tracked ticket.
