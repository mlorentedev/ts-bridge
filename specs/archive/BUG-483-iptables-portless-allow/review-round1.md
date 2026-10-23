---
spec: "BUG-483-iptables-portless-allow"
verdict: "FAIL"
reviewed_sha: "7325180a05698bd670f2ca141c6f7a34b1049e32"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-22"
---

## Adversarial review

**Scope**: `BUG-483-iptables-portless-allow` — diff `c249243076abdad068bebe072701779e204c2ffe...HEAD`
(3 commits, PR #493, `Closes #483`).
**Sources**: `specs/BUG-483-iptables-portless-allow/{proposal,tasks,verification,features}.md`;
`git diff c249243076abdad068bebe072701779e204c2ffe...HEAD`; `internal/host/linux_steps.go`,
`internal/host/linux_steps_test.go`, `docs/design/host-client-parity.md`,
`docs/lessons/lesson-046-2026-10-22.md`.
`review-request.json` contract digests match `dotf spec digest BUG-483-iptables-portless-allow`
(all three identical), so the review is against the contract that ships.

### Spec and task alignment

- AC1 (port-less blanket / interface / source / negated-loopback / `-p tcp` / comment reads
  present): implemented and covered by `TestIptablesCovers` subtests `port-less blanket`,
  `interface`, `source`, `tcp`, `negated loopback interface`, `negated loopback source`,
  `comment`, plus `TestSetupForLinux/iptables, port-less tailnet allow` and
  `TestCheckForLinux/iptables, port-less tailnet allow`. Verified green.
- AC2 (loopback / non-TCP / other-match reads absent): implemented and covered
  (`port-less loopback interface|source|destination`, `ipv6 loopback source`, `udp`, `icmp`,
  `negated tcp`, `state`, `limit`, `recent`, `drop`), plus the `changed` setup/check cases.
- AC3 (every BUG-035 case keeps its answer): verified by diff (only additions to the table)
  and by reverting `linux_steps.go` to the base commit — under base, exactly the 8 new
  port-less table cases and the two new setup/check cases fail; every pre-existing case still
  passes. Red-green is real, not asserted.
- Scope discipline: the diff is code + tests + the two docs the lesson/parity rules require.
  No unrelated changes. `ts-input` (#492), shadowing (#481), duplicates (#480), ip6tables,
  nftables-only, firewalld and the ufw backend are declared out of scope and genuinely untouched.
- Backend parity (the ticket's actual complaint) is achieved for the common shapes:
  `-i tailscale0 -j ACCEPT` and `-s 100.64.0.0/10 -j ACCEPT` now read like
  `ufw allow in on tailscale0` / `ufw allow from 100.64.0.0/10`.
- Repo gates I ran, fresh: `go build ./...`; `go test -race -count=1 ./...` → all packages ok;
  `GOOS=linux|windows|darwin go vet ./...` → clean; `golangci-lint run` → 0 issues.
  The `features.json` verification commands do select tests (24 / 48 / 44 passing cases).
- Not reachable from this repository: the `ubuntu:24.04` end-to-end run claimed in
  `verification.md` commits no script or transcript, so it cannot be re-run as written
  (**UNVERIFIED**). I substituted my own container measurement (below) for the `-S` shapes.

### Findings

The change's stated worst failure mode is a **silent** `ok` (setup reports "Firewall allows
TCP 3389" while RDP stays closed); `proposal.md` §Risks and `lesson-046` both say every unmodeled
case must fall back to **absent**, the visible failure. The two Major findings below are new
instances of exactly that mode, introduced by this change: the base commit appended the rule for
these rulesets, HEAD reports `ok` and appends nothing. Reproduced end-to-end through
`firewallSetupStep` with `iptables -S INPUT` lines taken verbatim from `ubuntu:24.04` (iptables
1.8.10, nf_tables) in this session.

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | firewall correctness | `iptPortlessCovers` allow-lists the `tcp` *module* but never inspects its options, so a rule that narrows by TCP flags covers the port although it cannot admit a new connection. `-A INPUT -p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK ACK -j ACCEPT` (the classic pre-conntrack "allow established" idiom, functionally equivalent to the `conntrack` rule this spec deliberately excludes) and `--tcp-flags FIN,SYN,RST,ACK SYN,ACK` (replies only) both read as covering. `host setup` then prints `Firewall allows TCP 3389 (iptables)`; base printed `Firewall rule added`. | Probe: `iptablesCovers(...,3389)=true`; `firewallSetupStep` → `outcome=ok`, message `Firewall allows TCP 3389 (iptables)`; base commit on the same input → `outcome=changed`. Real `-S` lines measured in `ubuntu:24.04`, iptables 1.8.10 (this session). | **UNTESTED** — `TestIptablesCovers/port-less tcp flags` covers only the `SYN` form; no case covers an ACK-only or `SYN,ACK` rule. | code (`--tcp-flags` must be parsed: mask must include SYN and ACK, comp must include SYN and exclude ACK; anything else is absent) + tests |
| Major | REAL | firewall correctness | The same branch relies on `-m` as the only source of narrowing, so non-module narrowing options are ignored: `-A INPUT -f -j ACCEPT` (fragments only) reads as covering. Reproduced: `outcome=ok` on HEAD, `changed` on base. The parser's positive rule is "port-less ACCEPT with nothing narrowing it", but it is implemented as "no port flag and no non-tcp/comment `-m`", which is weaker. | Probe as above, `-f` measured in the same container run. | **UNTESTED** — no `-f`/`--fragment` case exists in `TestIptablesCovers`. | code (reject unmodeled option tokens, not only unmodeled `-m` modules) + tests |
| Major | REAL | firewall correctness | `iptFlag` returns `""` for a **negated** flag and the port-less branch reads `""` as "no scope", so negation is silently discarded. `-A INPUT ! -s 100.64.0.0/10 -j ACCEPT` and `-A INPUT ! -i tailscale0 -j ACCEPT` both read as covering, although they exclude exactly the traffic that must reach RDP. AC1 legitimately wants `! -i lo` / `! -s 127.0.0.0/8` present (those negations exclude *loopback*), so the rule cannot be "any negation is absent" — it must be "a negation covers only when the negated value is loopback; otherwise absent". This also contradicts the change's own stated principle (lesson-046: guard the direction that fails silently; unmodeled → absent). | Probe: `iptablesCovers("! -s 100.64.0.0/10", 3389)=true`; `firewallSetupStep` → `ok`; base → `changed`. `! -s 100.64.0.0/10` prints that way in iptables 1.8.10 (measured). | **UNTESTED** — `port-less negated loopback source` is the only negation case, and it asserts `true`, which is the opposite direction. | code (only loopback negations count; other negations absent) + tests + spec (AC2) |
| Minor | THEORETICAL | firewall correctness | The loopback guard is applied only on the port-less branch. A port-bearing loopback rule — `-A INPUT -i lo -p tcp -m tcp --dport 3389 -j ACCEPT`, `-A INPUT -s 127.0.0.0/8 -p tcp -m tcp --dport 3389 -j ACCEPT` — still reads as covering (probe: `true`). Pre-existing BUG-035 behaviour and explicitly out of scope in `proposal.md` §What, so not a blocker here; but the guard is one `if` at the top of the `iptablesCovers` loop, and the identical silent-`ok` mode applies. | Probe on HEAD: both `true`. Same base behaviour, so no regression. | **UNTESTED** | code (hoist `iptLoopback` above the portless/port split; optional, follow-up) |
| Minor | SPECULATIVE | maintainability | `iptFlag`'s contract conflates "absent" with "negated" (both `""`), and both callers depend on that conflation in opposite directions: AC1 wants negation to count as unrestricted (`! -i lo`), while the narrowing exclusions want it to count as restricted. The second Major above is a direct consequence; the helper will keep producing this bug shape on the next reuse. The `iptPortlessCovers` doc comment ("matches only on tcp or comment") also over-promises relative to what the code checks. | Code read of `linux_steps.go:228-234` + probes above. | UNTESTED (design, not behaviour) | code (return an explicit negated/absent pair, or a small parse struct) — optional |
| Minor | THEORETICAL | verification | `verification.md` records the `ubuntu:24.04` end-to-end run as prose ("two `host setup` runs both report…") with no committed script or transcript, so the strongest piece of evidence in this spec cannot be re-run from the repository — the `host` package has no container harness. The unit-level claims *are* reproducible (I re-ran them), which is why this is Minor and not a correctness finding. | `specs/.../verification.md` §Test status; `ls scripts/tests/` shows no container harness for this path. | n/a (evidence) | spec (verification.md — outside the staleness-checked contract set) |

Refuted by measurement, recorded so it is not re-opened: `-p all` is *not* printed by
iptables 1.8.10 nf_tables — `iptables -A INPUT -p all -j ACCEPT` reads back as
`-A INPUT -j ACCEPT`, which the parser handles. The `-p 0`/legacy-backend variant remains
unmeasured (out of scope: legacy is not in this spec's supported set) — a question, not a finding,
and its failure direction is the visible one (`absent` → append).

No injection, secrets, auth, or concurrency exposure in this diff: the parser is pure string
work, no new dependency, no shell interpolation (rules are read as data; the append path is
unchanged `exec`-style argv). Complexity is well inside the bar (`iptablesCovers` ≈ 6,
`iptPortlessCovers` ≈ 5, `iptLoopback` ≈ 4; all under 40 lines).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | AC1/AC2 pass as written and AC3 is verified, but the new branch admits narrowing shapes (`--tcp-flags ACK`, `-f`) and discards negation through `iptFlag`, producing silent false `present` verdicts — a substantial negative-path gap, and a regression from base for those rulesets. |
| Verification       | B | Named tests are real and red-green verified by me, plus build/vet/lint; the container end-to-end is prose with no committed script, so it cannot be re-run. |
| Scope              | A | Diff matches the proposal exactly; the two doc updates are required by the repo's lesson/parity rules; nothing unrelated. |
| Reliability        | C | Read errors are handled and tested (`iptables -S fails` → failed step), but the port-less branch's failure direction is silent for every narrowing it does not model, which is the mode the spec set out to avoid. |
| Maintainability    | B | Small, clearly named functions with WHY comments; the `iptFlag` absent/negated conflation is the root of one Major and will recur. |
| Handoff-readiness  | A | Lesson-046, parity-table row, full spec triad, PR #493 with `Closes #483` and a `## Knowledge` section (Lesson named; ADR and Runbook `none:` with reasons). |

### Verdict

FAIL

Two **REAL Majors** (each a reproducible regression: base `changed` → HEAD `ok` with no rule
appended) and two Minor findings. Rubric has C and no D, which alone would give PASS WITH GAPS;
the severity × reality axis is the more severe path and decides.

### Recommended next steps

Minimum set to flip this to PASS (then re-review — the contract changes below invalidate this
verdict by design):

1. **Code** — make `iptPortlessCovers` require that the rule *admits a new connection* rather than
   merely carrying no port flag:
   - when `--tcp-flags` is present, parse its `mask comp` pair and accept only the `--syn` shape
     (mask ⊇ {SYN, ACK}, comp ∋ SYN, ACK ∉ comp); every other flag set is `absent`;
   - reject unmodeled narrowing option tokens (`-f`/`--fragment`), not only unmodeled `-m`
     modules;
   - a negated `-i`/`-s`/`-d` covers only when the negated value is loopback (`! -i lo`,
     `! -s 127.0.0.0/8`); any other negation is `absent`.
2. **Tests** — named regression cases, one per shape above:
   `TestIptablesCovers/port-less ack-only tcp flags`,
   `/port-less syn+ack tcp flags`, `/port-less fragment`,
   `/port-less negated tailnet source`, `/port-less negated tailnet interface`,
   plus a `TestSetupForLinux` case whose INPUT holds only an ACK-only `tcp-flags` rule and
   asserts `changed` (not `ok`).
3. **Contract** — narrow AC1's unqualified "`-m tcp --tcp-flags` reads as present" to the SYN
   form, and add the new absent cases to AC2. This is a contract-set edit
   (`proposal.md`), legitimate only because the verdict is FAIL; a re-review follows it.
4. **Non-blocking** (disposition in `verification.md` — applied, ticketed, or declined with a
   reason): hoist the loopback guard above the portless/port split so port-bearing loopback rules
   also read `absent`; give `iptFlag` an explicit absent/negated distinction; commit the
   container end-to-end as a script under `scripts/tests/` so the evidence is reproducible.

**Advisability of `dotf spec archive` in the current state**: **not advisable** — the gate refuses
this review anyway (`verdict: FAIL`), and archiving would record a parser that reports a closed
RDP port as allowed on at least five input shapes the base commit handled correctly.
