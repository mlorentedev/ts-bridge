---
spec: "HOST-005-host-config"
verdict: "PASS"
reviewed_sha: "8d8a73fb78e01c07c881a987908a4c89c6550cc4"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-05"
---

## Adversarial review

**Scope**: `HOST-005-host-config` (PR A1 #519, PR A2 #554, PR B #579, plus the two round-2 fixes #587 and #602)
**Sources**: `specs/HOST-005-host-config/{proposal,tasks,verification,features}.md`,
`review-request.json`, `review-round1.md` (the round-2 verdict), the diff
`243fdd5dfcd8568930496aa65f0a48d4e3facf26...HEAD` (`8d8a73f`), and the code it lands in:
`internal/host/{file,config}.go`, `cmd/cli/{host,host_init}.go`, `internal/profile/descriptor.go`,
their tests, and the docs.

**Range note**: `HEAD` is `origin/master` exactly (`git log origin/master..HEAD` is empty), so the
range is master's history since the commit HOST-005 started from, not only this spec's work. The
spec's own commits are `4363338` (A1, #519), `ecace2e` (A2, #554), `3e1a3e2` (B, #579), `e5adf4e`
(#587, AC6 JSON) and `7bf4cd9` (#602, release marker). The rest — HOST-008 `host serve`, HOST-455
Windows state, HOST-561 state-dir ancestors, CI-563, BUG-480/492/530, REL-376 — was judged only
where it touches this spec (the host file schema gained `host.mesh`; `host check --json` gained
`published`). Everything else in the range is **UNVERIFIED** by this review and out of its scope.

**Independence note**: I did not write this change and this is my first pass at it; the previous
round's verdict was a different run of the same model, and its two Majors were re-checked against
the merged artefacts rather than taken as fixed on the author's word.

### Spec and task alignment

- **Contract digests match.** `dotf spec digest HOST-005-host-config` reproduces
  `review-request.json`'s `contract_digests` byte for byte (features.json / proposal.md /
  tasks.md; `spec.md` empty, as this is a full spec). The review is against the current contract.
- `tasks.md` is fully ticked except "Lint passes" and "PRs opened referencing this spec folder" —
  the first is delegated to CI by the repo's own rule (endpoint security blocks the local linter,
  AGENTS.md), the second is the archive PR itself.
- Every AC1–AC8 maps to a named test that exists and passes in this tree (evidence table below).
- **Round-2 Major 1 (AC6 `setup --json` descriptor) is fixed in artefact, not just in prose.**
  `setupJSONOutput` now carries `Descriptor string \`json:"descriptor,omitempty"\``
  (`cmd/cli/host.go:506-515`), built by the same `buildDescriptorLine` as the text summary, and
  `printSetupJSON` now takes the IP. Two named tests prove it:
  `TestSetupJSON_DescriptorControlURL` (three cases: declared plane, `cp=saas`, key omitted with
  no IP) and `TestHostSetupCmd_DescriptorControlURL`, which drives the real `setup` command in
  both text and `--json` modes.
- **Round-2 Major 2 (missing release marker) is fixed.** `7bf4cd9` is `docs(host)!:` with a
  `BREAKING CHANGE:` footer naming the `--config` requirement and the `.env` retirement; both are
  on master (`HEAD`), so the next release-please run reads them from history. `verification.md`
  no longer claims #579 was retitled — it now states what merged and points at #602.
- Round-2 Minors are dispositioned in `verification.md` (non-Windows `--firewall-rule`/`--no-sleep`
  refusal, macOS `--config` exemption, `runHostSetup` → #588). Schema drift (`host.mesh`) is
  recorded rather than silently contracted.
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags in the spec folder (the only grep hits are inside
  this run's own `review-transcript.jsonl`).

### Evidence run (this session, this tree, `8d8a73f`)

| Command | Result |
|---|---|
| `dotf spec digest HOST-005-host-config` | identical to `review-request.json` |
| `go build ./...` | exit 0 |
| `go test ./... -count=1` | exit 0, all 15 packages `ok` |
| `gofmt -l .` | empty |
| `go vet ./...` | exit 0 |
| All eight `features.json` `verification` commands, executed verbatim | f1–f8 exit 0 |
| `go test ./cmd/cli -run TestHostInit -v` | `TestHostInit_RequiresConfig`, `_WritesYAML` (8 subtests incl. `a_missing_parent_is_created`, `windows-only flags on linux`, `darwin writes nothing`), `_Overwrite` (6 subtests incl. `a_directory is refused even with --force`, `an unreadable file is refused even with --force`) — all PASS |
| CLI probes with a binary built into `mktemp -d` | `control_url: https://user:pass@hs.example` → exit 1, `must not carry credentials`, **value not echoed**; a second YAML document → exit 1, named error; unknown `host.bogus` → one `WARNING … ignored` on stderr and the command proceeds; `vnc` on darwin → `host.services[0].kind "vnc": darwin has no vnc step in this version (#459)`; `power` on darwin → `host.power: darwin has no power step (ADR-017 §5)`; `host init` on darwin → "not applicable", exit 0, writes nothing |

Not reachable on this machine (darwin): the `host init` **write** path and Linux/Windows-only
capability rows. Those are covered by the named tests above (`TestHostInit_WritesYAML` runs the
linux/windows tables; `TestHostInit_Overwrite` needs a non-root Unix run, which `verification.md`
defers to the CI `test` job — I could not re-run it as root here, so its root/non-root split is
**UNVERIFIED**, while the subtests pass locally as a non-root user).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | evidence mapping | `features.json` f6 does not name either test that proves AC6's `setup --json` half (`TestHostSetupCmd_DescriptorControlURL`, `TestSetupJSON_DescriptorControlURL`), so f6 stays green if the descriptor disappears from `setup --json` again — the exact round-2 regression. `verification.md` does name them | f6's `verification` runs only `TestLoadFile_InvalidEntries`, `TestCheckFor_ControlURL`, `TestSetupFor_ControlURL`, `TestHostCmds_DescriptorControlURL`; the last drives `check` only | Named but outside f6's command: `TestHostSetupCmd_DescriptorControlURL` | features.json is the **contract set**: do not edit now. Record the mapping in `verification.md`, or carry it into the follow-up ticket |
| Minor | REAL | maintainability | `runHostSetup` is 98 lines against the repo's < 40 rule; it absorbed the `--config` load, the darwin ordering and the not-applied-port warning | `awk '/^func runHostSetup\(/,/^}/' cmd/cli/host.go \| wc -l` → 98 | n/a (quality) — UNTESTED | code, already ticketed as #588 |
| Minor | THEORETICAL | AC7 literal | On macOS `host init` returns before `parseHostInitFlags`, so `--config` is not required and `--force`/`--no-sleep` are accepted and ignored with exit 0 — AC7's "without it, the command fails before prompting" holds on Windows/Linux only | probe: `./tsb host init` and `./tsb host init --config cfg/host.yaml --force` both print "not applicable" and exit 0, nothing written | `TestHostInit_WritesYAML` / `darwin writes nothing` | already dispositioned in `verification.md`; round 1 asked for this ordering, so it is a decision, not drift |
| Question | — | verification honesty | `verification.md` "Test status" asserts the suite ran on Windows on 2026-11-03 and in WSL on 2026-11-03; I cannot reproduce either from here | this run: darwin, all green | — | `verification.md` (leave as-is or add "reproduced on darwin 2026-11-05") |

Not findings, checked and dismissed:

- `--port`/`TS_HOST_RDP_PORT` on a host whose file declares no `rdp` service only move the legacy
  `Config.Port`; the proposal phrases the rule as "set the `rdp` service's port", which presumes
  one exists. No user-visible inconsistency found (`check --json` reports the resolved port once).
- The capability error fires before the port-range error for an entry that is both unsupported and
  out of range (`rdp`/70000 on darwin names the OS, not the port). The proposal mandates no
  ordering between two valid rejections.
- Descriptor round-trip: `profile.Parse` accepts any `cp` value (it only special-cases `saas`), so
  a `control_url` with a path or query survives `String()` → `Parse()`; nothing to fix.
- Empty or comment-only `--config` file decodes to `version: 1` with no declarations, i.e. per-OS
  defaults. Consistent with "version absent is version 1" and with the file being optional.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | AC1–AC8 each proved by a named passing test, negative paths included (invalid entries, duplicates, credential URL, second document, unreadable path, per-OS refusal), and the CLI probes agree |
| Verification       | B | Digests, build, vet, gofmt and f1–f8 reproduce exactly; f6's named command lags the AC6 proof that now exists |
| Scope              | B | The spec's five commits match the proposal's A1/A2/B split; the range carries six other specs' work, judged only where it touches the host file |
| Reliability        | A | Write path is temp-file → load-back → rename, parent created `0755`, `0644` mode, every refusal leaves existing content untouched; validation precedes every step on both commands |
| Maintainability    | B | Clear naming, table tests, small helpers; `runHostSetup` at 98 lines is the one rule break, ticketed (#588) |
| Handoff-readiness  | B | `verification.md` records round-1/round-2 dispositions, lesson-049 promoted, ADR answered `no` with a reason; the false marker claim is corrected |

### Verdict
PASS

Both round-2 **REAL Majors** are fixed in the merged artefacts with named tests, and nothing in
this pass rises to a Blocker or a REAL Major. Rubric has no D and no C, so under the aggregation
rule the verdict is PASS. The open items are minors: one evidence-mapping gap inside the contract
set (do not edit it now), one ticketed maintainability break, one already-dispositioned AC7
literal reading, and one evidence claim I could not reproduce on this platform.

### Recommended next steps

All of these are **outside the contract set** — `proposal.md`, `tasks.md` and `features.json` are
closed for this verdict and must not be edited before archiving, or the archive will refuse this
review as stale:

- **`verification.md` (allowed):** add one line under AC6 naming `TestSetupJSON_DescriptorControlURL`
  and `TestHostSetupCmd_DescriptorControlURL` as the proof of the `setup --json` half, so the next
  reader does not have to rediscover that `features.json` f6 lags it. Optionally note "suite
  reproduced on darwin 2026-11-05" beside the Windows/WSL claims.
- **Follow-up ticket:** fold `TestSetupJSON_DescriptorControlURL` into `features.json` f6's
  `verification` command the next time the contract is opened deliberately (a new round, not an
  archive).
- **Archive is advisable** in the current state: `dotf spec archive HOST-005-host-config` should
  accept this review (matching `specs/HOST-005-host-config/review.md`, `reviewer:
  nan/deepseek-v4-flash`, `reviewed_sha: 8d8a73f`, digests current). Complete the archive checklist
  in `verification.md` (status `archived`, folder move, board ticket to Done) as part of that
  command, and keep `runHostSetup`'s split (#588) as the only carried debt.
