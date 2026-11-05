---
spec: "HOST-005-host-config"
verdict: "FAIL"
reviewed_sha: "42c777ba721001a01f1124b93c34327ccf90d1fc"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-04"
---

## Adversarial review

**Scope**: `HOST-005-host-config` (PR A1 #519, PR A2 #554, PR B #579)
**Sources**: `specs/HOST-005-host-config/{proposal,tasks,verification}.md`, `features.json`,
`review-request.json`; diff `243fdd5dfcd8568930496aa65f0a48d4e3facf26...HEAD` (`42c777b`);
`internal/host/{file,config}.go`, `cmd/cli/{host,host_init}.go` and their tests; docs
(`cli-reference.md`, `configuration.md`, `README.md`, `docs/design/host-client-parity.md`).

**Independence note**: the range also contains work merged from master between the launcher's
base and HEAD (HOST-008, CI-563, BUG-480/492/530, CI wiring). Those are judged only where they
interact with this spec; the spec's own commits are `4363338` (A1), `ecace2e` (A2), `3e1a3e2` (B).

### Spec and task alignment

- Contract digests: `dotf spec digest HOST-005-host-config` reproduces
  `review-request.json`'s `contract_digests` exactly (features.json / proposal.md / tasks.md).
  The review is against the current contract, not a stale one.
- AC1/AC2/AC4/AC5/AC7/AC8 map to named tests that exist and pass (see "Evidence run" below).
- **AC6 is only half implemented.** `proposal.md` *What*: "it becomes the `cp=` of the `tsb://`
  descriptor printed by `setup` and `check` (text and `--json`)". `setup --json` prints no
  descriptor: `setupJSONOutput` has `Steps`, `RDPPort`, `Warnings`, `Errors` and nothing else,
  while `checkJSON` carries `Descriptor`. `tasks.md` silently narrowed the AC to
  "`checkJSON`, `printCheckText` and `printSetupSummary`", dropping `setup --json` without a word
  in the contract — a spec↔task drift, not a decision recorded anywhere.
- AC3 (C9) is enforced two ways: `TestHostPackage_DoesNotImportUserStores` (source guard on
  `internal/host`, the only package that can reach the store) and
  `TestHostCmds_NeverReadUserStore` (behavioral guard over setup/check/serve/init, direct and
  through the root command, with a deliberately malformed store so a silent read fails rather
  than passes). `cmd/cli/host.go` does import `internal/profile`, but only for the pure
  `Descriptor` value; the behavioral guard covers it. Adequate.
- `verification.md` states: "Review round 1 … the breaking `host init` change lacked a release
  marker (PR #579 retitled `feat(host)!` with a `BREAKING CHANGE:` footer)." That claim is false
  on the merged artefact (finding 2).
- Docs are done as the proposal asks: `cli-reference.md` (init/setup/check/serve `--config`, the
  `0644` and traversable-parent reasoning), `configuration.md` (schema, precedence, mesh),
  `README.md` host section, and parity rows G1/G3/G5 updated to "Closed" against #458.

### Evidence run (this session, this tree)

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go test ./... -count=1` | exit 0, all 14 packages `ok` |
| `go test -race -count=1 ./internal/host ./cmd/cli` | exit 0 (`ok` both), corroborating the `-race` claim `verification.md` defers to CI |
| `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` | all exit 0 |
| `gofmt -l .` | empty |
| All eight `features.json` `verification` commands, executed verbatim | f1–f8 exit 0 |
| `dotf spec digest HOST-005-host-config` | identical to `review-request.json` |
| Loader probes via a binary built into `mktemp -d`: duplicate top-level `host:` key, `version:` with no value, `control_url: https://hs.example:0`, `services` with an invalid 2nd entry, CRLF file, key carrying `\x1b[31m` | duplicate key rejected ("mapping key \"host\" already defined"); `version:`/null rejected; port 0 rejected; entry named by index; CRLF accepted; the ANSI key is `%q`-escaped in the warning, **no terminal-control injection** |
| `tsb host check --config <absent>` and `tsb --config <absent> host check` | both exit 1 naming the path — the root-positioned `--config` is *not* silently ignored, so the shadowing is benign |
| `gh pr view 579 --json title`, `git log -1 --format=%B 3e1a3e2` | title and commit subject are `feat(host): …`; no `!`, no `BREAKING CHANGE:` anywhere in history |

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | contract / AC6 | `host setup --json` emits no descriptor, so a declared `host.control_url` never reaches setup's machine-readable output — the proposal's "(text and `--json`)" is only met for `check` | `setupJSONOutput` (`cmd/cli/host.go`) has no `Descriptor` field; `checkJSON` does; `buildDescriptorLine` is called only from `printSetupSummary` (text) and `printCheckText`/`checkJSON` | UNTESTED — `TestPrintSetupSummary_ControlURL` covers setup **text** only; no test asserts a descriptor in `setup --json` | code + tests (add `descriptor` to `setupJSONOutput`, assert it) — or narrow AC6 in `proposal.md`, which is a contract edit and therefore a re-review |
| Major | REAL | release / verification honesty | The breaking `host init` change carries no release marker. `verification.md` asserts PR #579 was retitled `feat(host)!` with a `BREAKING CHANGE:` footer; the merged PR title and the master squash commit are both plain `feat(host):`, so release-please files the break under *Features* | `gh pr view 579` → `feat(host): host init writes the YAML host file named by --config`; `git log -1 --format=%B 3e1a3e2` → no `!`, no footer; `release-please-config.json` (`release-type: go`) derives `CHANGELOG.md` from commits; `proposal.md` Risks relies on "the CHANGELOG entry flags it" | UNTESTED — no gate ties the breaking CLI change to the marker | code/repo (a commit carrying the breaking marker, or a recorded breaking-change note the release reads) + `verification.md` corrected to reality |
| Minor | REAL | scope of behaviour change | `host init --firewall-rule` / `--no-sleep` are now refused on non-Windows, but `proposal.md` says "Flags and env keep today's behaviour on every OS (for example, `--no-sleep` on Linux stays accepted and has no effect)"; before this change `host init --no-sleep` on Linux wrote `TS_HOST_NO_SLEEP=true` into `.env` | `TestHostInit_WritesYAML` / `windows-only flags on linux` asserts the refusal; old `cmd/cli/host_init.go` at `243fdd5` wrote the variable unconditionally | `TestHostInit_WritesYAML` (subtest `windows-only flags on linux`) | spec (`verification.md` disposition + follow-up note); the refusal itself is the safer choice and `verification.md` already records the reasoning |
| Minor | THEORETICAL | AC7 literal reading | On macOS `host init` returns before `parseHostInitFlags`, so `--config` is *not* required there, while AC7 says "Without it, the command fails before prompting, naming the flag" | `runHostInit` checks `hostGOOS == "darwin"` first; `TestHostInit_WritesYAML` / `darwin writes nothing` asserts exit 0 without `--config` | `TestHostInit_WritesYAML` (subtest `darwin writes nothing`) | spec (`verification.md` disposition — round 1 asked for this ordering, so record it as decided rather than drift) |
| Minor | REAL | maintainability | `runHostSetup` is 101 lines (repo standard: < 40); this change grew it from 92 by adding the `--config` load, the darwin ordering and the not-applied port warning | `cmd/cli/host.go`; base version at `243fdd5` measured at 92 lines | n/a (quality, not behaviour) | code (extract the resolve/warn/print blocks), follow-up ticket |

Not findings, checked and dismissed: the loader's unknown-key warning goes through `%q`, so a key
cannot inject terminal control sequences; duplicate top-level keys are rejected by the map decode;
`version: 1.0` is accepted as `1` (benign, undocumented, arguably correct); `features.json`'s
`"state": "pending"` is the repo norm (111 pending vs 12 passing across specs, and
`scripts/check-features-shape.sh` passes), so it is not evidence rot.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | AC1-A5, A7, A8 verified by named passing tests; AC6 met for `check` (text+JSON) and setup text, **not** for `setup --json` |
| Verification       | C | Reproducible commands and digests check out, but one evidence claim in `verification.md` is false and AC6's JSON half has no test |
| Scope              | B | The spec's three commits match the proposal split; #579 also carried `lesson-052` from the HOST-008 work (documented in its body); the bulk of the range is other PRs merged from master, not this branch's scope creep |
| Reliability        | A | Write goes through a same-directory temp file, is loaded back with the real validator, then renamed; dir/unreadable paths refused even with `--force`; existing content untouched on every refusal path |
| Maintainability    | C | Clear naming and table-driven tests, but `runHostSetup` is 101 lines against a < 40 standard and absorbed three unrelated concerns |
| Handoff-readiness  | B | Spec artifacts, docs and lesson-049 updated and AC mapping present; the false marker claim and the silently narrowed AC6 keep it below A |

### Verdict
FAIL

Two **REAL Majors**: AC6 is unmet in `setup --json` (UNTESTED), and a breaking CLI change is
shipping without the release marker `verification.md` says it has — which removes the mitigation
`proposal.md` depends on for accepting the break. Neither is a Blocker, but a REAL Major forces
FAIL under `severity × reality`. Rubric agrees: Correctness C and Verification C.

### Recommended next steps

- **Contract set (closed; changes here need the next round to re-review):** decide AC6's JSON
  half. Implementing `descriptor` in `setupJSONOutput` is additive and needs no contract edit —
  prefer it; only if setup's JSON is deliberately out of scope, narrow AC6 in `proposal.md` and
  say why in `verification.md`.
- **Code + tests:** add `TestHostSetupJSON_DescriptorControlURL` (declared `control_url` appears
  as `cp=` in `setup --json`, absent when no file declares one). That is the named regression the
  AC6 gap lacks.
- **Repo/release (outside the contract set, apply now):** give the breaking `host init` change a
  marker the release actually reads — a commit subject `feat(host)!` or a `BREAKING CHANGE:`
  footer — or, if master's history is not being rewritten, a short breaking-change entry in
  `docs/troubleshooting/` that the release notes link. Then correct the `verification.md`
  round-1 bullet to state what is true.
- **`verification.md` dispositions (no contract edit):** record the two Minor divergences above —
  the non-Windows flag refusal and the macOS `--config` exemption — as "applied, decided on
  review of round 1" or "declined: <why>", so they are decisions rather than drift.
- **Follow-up ticket:** split `runHostSetup` (currently 101 lines) as debt, per the repo's
  function-length rule.
- **Not ready to archive.** The `## Archive checklist` in `verification.md` is unticked, and
  `dotf spec archive` will refuse this review by verdict ("FAIL"); it needs the two Majors
  addressed and a fresh round.
