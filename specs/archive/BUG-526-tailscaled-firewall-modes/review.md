---
spec: "BUG-526-tailscaled-firewall-modes"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "9c96451a1d6a7bb409392ea5b6378ae8d6b6b6ce"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-11"
---

## Adversarial review

**Scope**: `BUG-526-tailscaled-firewall-modes` — the whole range
`949ba4bc202099520eafb88064ca7403d0d19483...HEAD` (`9c96451`). The branch carries two squash
commits on that base: `8814575` (`#714`, this spec's implementation) and `c3a7a58` (`#719`,
the separate `HOST-011-control-liveness` spec), plus `9c96451` (spec status). `#719` is judged
only where it interacts, and it does not: it touches `internal/serve/` and
`cmd/cli/host_serve.go`, never `internal/host/linux_steps.go`. This is **round 2**: round 1
(`review-round1.md`, `8d832f2`) returned PASS-WITH-GAPS, and its dispositions are re-verified
below against the state that actually landed.
**Sources**: `specs/BUG-526-tailscaled-firewall-modes/{proposal,tasks,verification}.md`,
`features.json`, `review-request.json`, `review-round1.md`, the diff above, and re-run evidence:
`go test -race ./...`, `golangci-lint run ./...` (0 issues), `gofmt -l .` (empty), `go vet ./...`
plus `GOOS=linux`/`GOOS=windows` on `internal/host`, `shellcheck -x` on the seven touched scripts,
the four `features.json` replays I could run, four mutants and twelve rule-shape probes in a
throwaway clone under `mktemp -d`.

### Spec and task alignment

- Contract digests in `review-request.json` equal `dotf spec digest
  BUG-526-tailscaled-firewall-modes` exactly, all four (`features.json` `a8ded60e…`,
  `proposal.md` `36ad80be…`, `tasks.md` `57394882…`, `spec.md` empty). This is why a round 2
  exists: `features.json` `f1` changed after round 1, and the digest is what the gate compares.
- `tasks.md`: every box is `[x]`, and each ticked implementation/closing box is backed by the
  diff — the nodivert/nftables capture fixtures and `TestReadIptablesCapturedModes`, the
  `ip6Exposed` read plus `TestIP6Exposure`, the `scripts/e2e/linux.sh` v6 block, the docs edit,
  and `verification.md` with the command output it claims.
- Acceptance criteria, re-verified rather than taken on trust:
  - **AC1** `TestReadIptablesCapturedModes/netfilter-mode_nodivert` passes (state `ruleUnknown`,
    reason names `nodivert`) and `TestFirewallStepsTailscaledModes/nodivert` passes, which is the
    "setup writes nothing" half that round-1 finding 5 asked to cite. Both are now cited.
  - **AC2** `TestReadIptablesCapturedModes/nftables_mode,_read_through_iptables-nft` passes on the
    captured `-i tailscale0+` chain.
  - **AC3/AC4** `TestIP6Exposure` passes all ten rows. I reproduced the three mutants
    `verification.md` claims: dropping `len(fw.v6Foreign) == 0` from `linuxFirewallOK`, dropping
    `withIP6` from the setup step and dropping it from the check step each turn it red. A fourth
    mutant (removing the `::/0` exception in `ip6Narrowed`) reds the two `::/0` rows, so the
    round-1 fix is genuinely pinned, not merely present.
  - **AC5** is the one criterion I could **not** verify: `scripts/e2e/out/` is gitignored
    (`.gitignore:76`) and `linux-20261111T071101Z.log` is not in the repo. The `features.json`
    `f5` command fails closed when the log is absent, so it is not vacuous, but its claim rests
    on the transcript alone. I read the e2e block instead: `fwstep` and the `manual` outcome
    string it asserts both exist (`scripts/e2e/linux.sh:153`, `internal/host/step.go:20`), and the
    four cases match AC5's two halves. Marked UNVERIFIED, not wrong.
- Round-1 dispositions hold up: `ip6Exposed`/`ip6Narrowed` replaced the reuse of `iptUnscoped`
  (finding 1), `toolSaid` now quotes the tool's first stderr line (finding 3,
  `TestReadIP6NoteQuotesTheTool` passes), `TestIP6Exposure` asserts the check path's call
  sequence as well as the setup path's (finding 4), and finding 2's decline is recorded with a
  reason that matches the code — my probe of nodivert-v4 + an unscoped v6 allow returns
  `unverified`, `firewall_ok=false`, and the message names the v6 rule.
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` remain. Promotion lines are all answered; lesson 055
  exists and is corrected; no ADR was claimed.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location |
|----------|---------|------|---------|----------|---------------------------|--------------|
| Major | THEORETICAL | exposure scan / correctness | A v6 allow with a **negated destination port** covers the port on every interface, and the scan does not see it: `-A INPUT -p tcp -m tcp ! --dport 22 -j ACCEPT` admits 3389 everywhere, but `iptPortCovers` reads the port through `iptFlag`, which returns `""` for a negated flag, so `ip6Exposed` returns nothing and `check` reports `ok` with `firewall_ok: true`. That is the failure the spec exists to remove ("reports a host converged while it is exposed over IPv6"), on a rule shape an operator plausibly writes ("everything but SSH"). It is **not a regression**: the v4 exposure predicate `iptUnscoped` shares `iptPortCovers`, so the same shape is invisible in v4 today — which is why this is a Major to track, not a Blocker against this change. | Probe in a throwaway clone: `-A INPUT -p tcp -m tcp ! --dport 22 -j ACCEPT` → `exposed=false`; the same line with `--dport 3389` → `exposed=true`; `-m multiport --dports 3389,22`, `--dport 3389:3390` and `-m conntrack --ctstate NEW` all → `exposed=true` (those shapes are handled). `iptPortCovers`/`iptFlag` at `internal/host/linux_steps.go:679` and `:760`; `iptUnscoped` at `:657` shares it. | UNTESTED — `TestIP6Exposure` has no negated-port row, and `TestReadIptablesUnscoped` covers the v4 path only | code (treat a negated destination port as covering the port in the exposure scan, and mirror it in the v4 predicate) + tests (one named `TestIP6Exposure` row); or a bitácora ticket if it is deferred, since `#720` covers jumped chains only |
| Minor | REAL | code quality | `toolSaid` was inserted between `chainMissing`'s doc comment and `chainMissing` itself, so `toolSaid`'s godoc reads "chainMissing reports a failure that says the chain does not exist…" and `chainMissing` has no doc comment at all. A reader of either function is now told the wrong thing. | Read of `internal/host/linux_steps.go:483-502`: the five-line `chainMissing` comment sits directly above `func toolSaid`, and `func chainMissing` follows `toolSaid` bare. The insertion is in this diff. | UNTESTED (no test reads doc comments, and that is fine) | code (move the comment back above `chainMissing`, give `toolSaid` its own) |
| Minor | REAL | docs vs behaviour | Lesson 055's correction says `host check`/`host setup` "scan the v6 `INPUT` chain for an allow that opens the port **off the tailnet** and report it as `manual`". The code reports only *unscoped* allows: a v6 rule scoped to another interface — `-A INPUT -i eth0 -p tcp -m tcp --dport 3389 -j ACCEPT`, open to the whole LAN — reads `ok`. The contract is precise here ("no interface, source or destination") and `docs/design/host-client-parity.md` is too; the lesson is the one document an operator will read and it promises more than the code does. | Probe: `-i eth0 …` → `exposed=false`; `-s 2001:db8::/32 …` → `exposed=false`; `-s ::/0` and `! -i tailscale0` → `exposed=true`. `TestIP6Exposure/a_v6_allow_for_one_LAN_prefix_is_the_operator's_scope` pins the deliberate behaviour. Lesson text at `docs/lessons/lesson-055-2026-11-04.md` (last paragraph). | `TestIP6Exposure/a_v6_allow_for_one_LAN_prefix_is_the_operator's_scope` proves the behaviour; no test can prove the prose | docs (`lesson-055`, say "an unscoped allow", matching the parity table) |
| Minor | REAL | e2e harness coverage | `scripts/e2e/capture-tsfw.sh` is a new 54-line driver with a VM lifecycle, a build, a teardown trap and shared Headscale helpers, and it has no shim test in `scripts/tests/`, unlike its siblings (`test-e2e-run-teardown.sh`, `test-e2e-headscale.sh`, `test-e2e-mesh-preflight.sh`, all run by the `hygiene` job). AC1/AC2 rest on the fixtures it produces, and those fixtures' "verbatim" provenance cannot be re-checked from the repo at all: the log it writes is gitignored. | `ls scripts/tests/` has no capture case; `grep -rn capture-tsfw scripts/tests .github/workflows` finds nothing; `git check-ignore` confirms `scripts/e2e/out/`. | UNTESTED — no shim covers the driver | tests (a `scripts/tests/test-e2e-capture.sh` shim: `dotf`/`docker` absent must `die` 2, the teardown trap must run on a mid-script failure) |
| Minor | THEORETICAL | exposure scan | The scan is order-independent: an unscoped ACCEPT that appears *after* an unscoped DROP for the port is unreachable but is still reported `manual` with `firewall_ok: false`. Over-reporting is the safe direction and matches the v4 exposure read, so the cost is an operator deleting a dead rule. | Code read of `ip6Exposed` (no rule-order walk, unlike `iptablesDecide`). | UNTESTED | code (optional; or note it beside the v4 read's own comment) |
| Minor | REAL | verification artifact | AC5's proof is a gitignored log, so `features.json` `f5` replays only on the machine that ran the VM. The command fails closed when no log exists, so nothing is silently green — but a fresh clone cannot reproduce the AC, which is what the rubric's Verification dimension asks for. | `.gitignore:76`; `ls scripts/e2e/out/` empty in this worktree; `f5`'s command begins `l=$(ls -t scripts/e2e/out/linux-*.log | head -1)`. | UNTESTED (by construction: it is the evidence itself) | verification.md (record that the log is deliberately not committed and how a reviewer reproduces it: `make e2e-linux`) |
| Question / assumption | SPECULATIVE | fixture provenance | The captured `ts-input` fixture carries no magicsock UDP rule, while the pre-#526 fixture did (`-A ts-input -p udp -m udp --dport 41641 -j ACCEPT`) and the capture script itself fixes the port (`defaults()` writes `PORT="41641"`, which the guest's unit passes as `--port`). In `tailscale.com@v1.104.4`, `updateMagicsockPort` appends `-p udp --dport <bound port> -j ACCEPT` to `ts-input` whenever the published port is non-zero, and the linux router always subscribes to `router.PortUpdate`. Either the published port was 0 in that run (a DERP-only or pre-bind capture) or the fixture is not verbatim. It cannot change the read outcome — a UDP rule is skipped by both `iptablesDecide` and `ip6Exposed` — but AC1/AC2 are pinned on fixtures whose "verbatim" claim no reader can re-derive from the repo. **Surfaced only: a SPECULATIVE finding does not move this verdict.** | `internal/host/linux_steps_test.go` (`tsInputDefault`, `tsInputNFTMode`); `scripts/e2e/tsfw_linux.sh` (`PORT="41641"`); `$GOMODCACHE/tailscale.com@v1.104.4/wgengine/router/osrouter/router_linux.go:688-737`, `util/linuxfw/iptables_runner.go:640` | UNTESTED (needs a VM; `make e2e` is the only reproduction) | spec artifacts (`verification.md`: say what the capture run's `--port` was, or name the log line that shows the rule absent) |

Code-level checklist, walked over the diff: **injection** — the v6 rule text is interpolated into a
message and a `Fix` string, never into a command; `setup` still runs only `iptablesAppend`/
`ufwAllowArgs` from its own constants, and my probe of a rule with a quoted `--comment` shows the
quoted text surviving into the `-D` fix unexecuted. **Secrets** — the capture harness reads the
auth key from stdin into a 0600 file, never argv; nothing is printed. **Auth** — unchanged.
**Performance** — one extra bounded `ip6tables -S INPUT` per elevated read, gated on `LookPath`, no
loop over hosts. **Resilience** — failed and absent v6 reads are notes, not failures, and nothing is
deleted; the one gap is finding 1. **Quality** — `readIP6` ≈4, `ip6Exposed` ≈5, `ip6Narrowed` ≈2,
`withIP6` ≈5, `linuxFirewallOK` 1; all well under 10, under 40 lines, no dead code; the only defect
is the misattached comment (finding 2).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1–AC4 verified by named tests and four mutants; AC5's own log is unreachable; the v6 scan still has one false-negative shape (finding 1) |
| Verification       | B | I re-ran the suite, the lint, the vet, the four replayable `features.json` commands and the mutants; AC5 cannot be reproduced from the repo because its log is gitignored |
| Scope              | B | The spec's diff matches the proposal exactly, but the range also carries the separate `#719` (`HOST-011`) change as a squash commit — declared, separately specified, and with no interaction with this code |
| Reliability        | B | Failed/absent v6 reads are notes, nothing is deleted, unprivileged reads run nothing; the safety scan's one false negative is finding 1 |
| Maintainability    | B | Small single-purpose functions, table-driven tests, lint and shellcheck clean; `chainMissing` lost its doc comment to `toolSaid` (finding 2) |
| Handoff-readiness  | B | Verification records every decision and promotion, parity table matches the code; lesson 055 overstates what the scan reports (finding 3) |

### Verdict

PASS WITH GAPS

No Blocker, no REAL Major, no rubric C or D. The one Major is THEORETICAL and pre-existing in the
v4 read as well, so it is tracked rather than blocking. Round 1's substantive finding is fixed and
pinned by a mutant; its declined minor has a reason that matches the code. Five minors and one
question are open, each with a fix location outside the contract set.

`dotf spec archive` is **advisable** in this state: this review is fresh for the recorded contract
digests (checked with `dotf spec digest`, all four values equal) and `reviewer` is the pool's own
spelling of `nan/deepseek-v4-flash` in `harness/reviewer-pool.json`. `proposal.md` declares no
`risk: high`, so no `--second` signature is required. The archive gate's implementation lives
outside this repository, so its acceptance is reasoned from those pre-flights, not observed — I did
not run it, because it moves the folder and has no dry-run.

### Recommended next steps

The contract set (`proposal.md`, `tasks.md`, `features.json`) is closed by this verdict: editing
any of them invalidates it. Every item below lands outside that set and can be dispositioned in
`verification.md` as applied, ticketed, or declined with a reason.

- **Finding 1 (code + tests, or ticket):** treat a negated destination port as covering the port
  in the v6 exposure scan, and add one `TestIP6Exposure` row for `! --dport`. If it is deferred,
  file a bitácora issue naming the root cause — `#720` covers jumped chains only, so this shape is
  currently untracked — and record the decision in `verification.md`.
- **Finding 2:** move the `chainMissing` doc comment back onto `chainMissing` and give `toolSaid`
  its own line.
- **Finding 3:** change lesson 055's "off the tailnet" to "unscoped", matching the parity table and
  the contract.
- **Finding 4:** add the `scripts/tests/test-e2e-capture.sh` shim, so the capture driver's
  `dotf`/`docker` preflights and its teardown trap are exercised in CI like its siblings — or state
  in `verification.md` why a one-shot capture tool is exempt from that convention.
- **Finding 5:** record in `verification.md` that `features.json` `f5` needs the uncommitted
  `scripts/e2e/out/linux-*.log`, and that `make e2e-linux` is the reproduction.
- **Finding 6 (optional):** note beside `ip6Exposed` that the scan is order-independent, as
  `iptUnscoped` already is.
- **Question 7 (fixture provenance):** state in `verification.md` which `--port` the capture run
  used, or quote the `iptables -S ts-input` block from the log, so the fixture's "verbatim"
  claim has a check behind it. No code change is implied.
