---
spec: "BUG-483-iptables-portless-allow"
verdict: "PASS"
reviewed_sha: "ae9c58006501bebe387f6af7b5be5ac6e5f666bb"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-22"
---

## Adversarial review

**Scope**: `BUG-483-iptables-portless-allow` — diff
`c249243076abdad068bebe072701779e204c2ffe...HEAD` (5 commits, PR #493, `Closes #483`). This is
round 2: the base is unchanged, so the whole change is re-judged, including the round-1 fixes
against the code they sit next to.
**Sources**: `specs/BUG-483-iptables-portless-allow/{proposal,tasks,verification}.md`,
`features.json`, `review-request.json`; `git diff c249243...HEAD`; `internal/host/linux_steps.go`,
`internal/host/linux_steps_test.go`, `docs/design/host-client-parity.md`,
`docs/lessons/lesson-046-2026-10-22.md`.
Contract digests equal `dotf spec digest BUG-483-iptables-portless-allow` (all three), so this
review is against the contract that ships.

### Spec and task alignment

- **AC1** (port-less blanket / interface / source / negated-loopback / `-p tcp` / `--syn` /
  comment reads present): implemented; each shape has a named `TestIptablesCovers` subtest
  (`port-less blanket`, `port-less interface`, `port-less source`, `port-less tcp`,
  `port-less negated loopback interface|source`, `port-less tcp flags`, `port-less comment`) plus
  `TestSetupForLinux/iptables,_port-less_tailnet_allow` (no `-A`, `ok`) and
  `TestCheckForLinux/iptables,_port-less_tailnet_allow` (`ok`). Green, fresh.
- **AC2** (loopback / non-TCP / other-match reads absent): implemented; named coverage for
  loopback by interface, source, destination and `::1/128`; `udp`, `icmp`, `negated tcp`; `state`,
  `limit`, `recent`, `conntrack`; `fragment`; three `--tcp-flags` non-`--syn` forms; two
  non-loopback negations. Setup/check cases prove the `-A` is actually appended
  (`iptables, only loopback and established allowed`, `... only an established-traffic tcp-flags
  allow`).
- **AC3** (every BUG-035 case keeps its answer): verified by mutation, not by assertion — I put the
  base commit's `linux_steps.go` back and ran the current tests: exactly the ten new/changed
  `TestIptablesCovers` cases and the two new setup/check cases fail, every pre-existing case still
  passes. Red-green for the round-1 fixes likewise: restoring the round-1 implementation
  (`7325180`) turns exactly `port-less ack-only tcp flags`, `... syn+ack tcp flags`, `... other tcp
  flags mask`, `... fragment`, `... negated tailnet source`, `... negated tailnet interface`,
  `loopback interface with the port`, `loopback source with the port` and
  `TestSetupForLinux/... established-traffic tcp-flags allow` red. Both claims in `verification.md`
  hold.
- **Test fixtures are real iptables output, not fiction.** I ran `ubuntu:24.04` with iptables
  v1.8.10 (nf_tables) and `NET_ADMIN`, added every fixture shape, and read `iptables -S INPUT`.
  All match the test tables verbatim, including `--syn` → `-p tcp -m tcp --tcp-flags
  FIN,SYN,RST,ACK SYN`, `-f` printing as `-f`, `! -s 100.64.0.0/10` printing negated, `-d
  127.0.0.1` gaining `/32`, and a single-word comment printing unquoted.
- **Direction of every new failure is the visible one.** The port-less branch is an allow-list
  (`iptPortlessToken`), so an unmodeled shape falls to `absent` → setup appends and reports
  `changed`. The loopback guard only produces `absent`. I probed a set of shapes the table does
  not cover (`-m tcp` before `-p tcp`, `--dport` before `-p`, `--syn` plus a comment, a quoted
  comment containing `--dport`, a negated `--syn`, `--sport` only, `-m conntrack --ctstate NEW`,
  other port lists, `--tcp-flags ... ALL-mask SYN`): every one lands on the safe answer.
- **One declared behaviour change worth naming.** Hoisting the loopback guard above the port split
  means `-i lo --dport 3389` (and `-s 127.0.0.0/8 --dport 3389`) now reads `absent` where the base
  read `present`. On a host whose only allow is that rule, setup now appends the unrestricted
  `--dport 3389` rule instead of silently reporting `ok`. That is `proposal.md` AC3 as written
  ("BUG-035 had no case for it"), it is pinned by `loopback interface with the port` /
  `loopback source with the port`, and trading a silent lie for a visible widening is the
  documented intent of the change as a whole. Not a mismatch.
- **Scope discipline**: code + tests + the two docs the repo's lesson/parity rules require. No
  unrelated changes. `ts-input` (#492), shadowing (#481), duplicates (#480), ip6tables,
  nftables-only, firewalld and the ufw backend are declared out of scope and genuinely untouched.
- **No tests deleted or weakened** — the diff adds 30 table cases and 3 setup/1 check cases; the
  only edits to existing text are the fixture-constant comment and two fixture constants that kept
  their values. Nothing got quieter.
- Gates I ran fresh this session: `go build ./...` (exit 0), `go test -race -count=1 ./...` (all
  packages ok), `GOOS=linux go vet ./...` (clean), `golangci-lint run` (0 issues). No
  `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags in the spec files.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | THEORETICAL | loopback detection | `iptLoopbackScope` decides loopback only via `netip.ParsePrefix`, so a bare loopback literal (`-s 127.0.0.1`, `-s ::1`) is not recognised and the rule reads as covering — the silent `ok` this change exists to remove. iptables 1.8.10 (nf_tables) always prints the prefix (`-d 127.0.0.1` → `-d 127.0.0.1/32`, `::1` → `::1/128`), which I reproduced in `ubuntu:24.04`, so no reachable input produces it today. | probe of `iptablesCovers("-A INPUT -s 127.0.0.1 -j ACCEPT")` → `true`; container `-S` output shows the prefix | UNTESTED | code (also accept `netip.ParseAddr(...).IsLoopback()`) + tests — or decline with the measurement as the reason |
| Minor | SPECULATIVE | scope qualifiers | A non-loopback `-d <prefix>` and a non-loopback negated `! -s`/`! -d` count as covering, so an INPUT rule naming a destination that is not this host's address would be a false `ok`. It matches BUG-035's "the operator's scope" judgment and AC1 lists `-d <prefix>` as modeled, so this is a declared decision rather than drift. | probe of `-A INPUT -d 100.64.0.0/10 -j ACCEPT` → `true`; `proposal.md` AC1, §Risks | UNTESTED (the port-less `-d` shape has no case) | spec (declared; no action required) |
| Minor | REAL | verification | `verification.md`'s end-to-end claim (`host setup` twice on a tailnet-allow host, once on a loopback-only host, in `ubuntu:24.04`) commits no script or transcript, so it cannot be re-run from the repo. Round 1 raised this; it was declined with a reason (no container harness for `internal/host`). I reproduced the load-bearing half — that every fixture is real `-S` output — but not the E2E run. | container run this session; `verification.md` §Test status | UNTESTED (E2E) | tests (a committed script, its own ticket) — the decline reason already stands in `verification.md` |
| Minor | REAL | handoff | `verification.md` carries no `## Promotion candidates` section, which `dotf spec archive`'s third pre-flight reads ("each line must be answered ... or `no: <reason>`"), and which every sibling and archived spec carries. Separately, `features.json` still holds `state: pending` with empty `evidence` although `tasks.md` is fully ticked. Neither changes behaviour; both can refuse or confuse the archive step. | `dotf spec archive --help`; `grep "Promotion candidates"` matches 5+ other specs, none here | N/A | spec (`verification.md` is outside the contract set; `features.json` state/evidence are normalised out of the digest — confirm with `dotf spec digest` before archiving) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | All three ACs verified by named tests, mutation-tested against both the base and round-1 implementations, with fixtures confirmed against real iptables output and every probed uncovered shape falling to the safe answer. |
| Verification       | B | Every unit claim is reproducible by command and I re-ran them; the E2E container claim is prose-only and remained UNVERIFIED. |
| Scope              | A | Diff matches the proposal and its out-of-scope list exactly; docs touched are the ones the repo's own rules require. |
| Reliability        | A | Failure direction is fail-to-absent (visible `changed`, appended rule) for every unmodeled shape; no new error path, no I/O change. |
| Maintainability    | A | Short functions (`iptablesCovers`, `iptPortlessCovers`, `iptPortlessToken`, `iptLoopback` all well under 40 lines), the allow-list is the readable statement of intent, comments say why the allow-list errs to absent; `golangci-lint` clean with gocyclo enabled. |
| Handoff-readiness  | B | Lesson 046 and the parity row are updated and #492 is filed as the next step, but the archive bookkeeping (promotion candidates, `features.json` state) is not finished. |

### Verdict

PASS

No Blocker, no REAL Major. The three round-1 REAL Majors are fixed and each is pinned by a test
that goes red on the round-1 implementation — verified by mutation in this session, not taken from
`verification.md`. Remaining findings are Minors: one THEORETICAL, one SPECULATIVE, two
bookkeeping. Rubric is all B or above.

### Recommended next steps

The contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed** by this verdict — do not
edit it, or the archive gate will treat this review as stale and refuse it. Everything below is
outside that set or a follow-up ticket. Each line is for the implementer to disposition in
`verification.md`.

- **`verification.md`** (non-contract): add a `## Promotion candidates` section — lesson 046 exists,
  so `yes: docs/lessons/lesson-046-2026-10-22.md` is the expected line, plus `no: <reason>` for
  anything else — and record the disposition of the four findings above (apply / ticket /
  decline-with-reason). Round 1's decline of the E2E script already stands; keep it.
- **`features.json`** `state`/`evidence`: fill them from the test runs if the archive flow expects
  it, but confirm with `dotf spec digest BUG-483-iptables-portless-allow` against
  `review-request.json` afterwards — if the digest moves, that edit is inside the contract set and
  needs a re-review instead.
- **Optional hardening (ticket, not a blocker)**: accept a bare loopback address in
  `iptLoopbackScope` (`netip.ParseAddr`) alongside `ParsePrefix`, with a table case, so the guard
  does not depend on iptables' prefix normalisation. The measurement in `proposal.md` and this
  review says no current iptables emits the bare form, so this is defence in depth.
- **Archive**: `dotf spec archive BUG-483-iptables-portless-allow` is advisable once the
  bookkeeping above is done. The gate reads this review's frontmatter and the contract digests; the
  verdict is `PASS`, so no `--force-without-review` is needed or justified.
