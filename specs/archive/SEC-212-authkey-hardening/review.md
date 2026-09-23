---
spec: "SEC-212-authkey-hardening"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "00c71b92a904b4eaff0a717705d94afa2222ff05"
reviewer: "nan/deepseek-v4-flash"
date: "2026-09-22"
---
## Adversarial review

**Scope**: SEC-212-authkey-hardening — documentation/CLI hardening that promotes `--auth-key-file`
over a plaintext `TS_AUTHKEY` / inline `--auth-key`.
**Sources**: `specs/SEC-212-authkey-hardening/{proposal,tasks,verification}.md`;
`git diff 64d8a7774368dc90b1b6aae630309fa9b438bfa0...HEAD` plus
`git log 64d8a77..HEAD`; the SEC-212 slice of that range is
`a665485` (#298), `f1fcb0f` (#310), `00c71b9` (#347) for the spec files and
`cmd/cli/init.go`, `cmd/cli/init_test.go`, `README.md`, `.env.example`,
`docs/runbooks/guide-{deployment-windows,multi-device-operations}.md`,
`docs/troubleshooting/security-audit.md`. The same range also carries four *other* specs' work
(CI-314, CI-322, ARCH-186 and dependency/hygiene PRs) — judged only where it touches this spec's
subject matter, and attributed to this spec nowhere else.

Verification performed in this session (nothing taken from `verification.md` on trust):

```
go build ./...                                  OK
go vet ./...                                    clean
go test -race ./...                             10/10 packages ok (cmd/cli 1.655s)
GOOS=windows GOARCH=amd64 go build ./...        OK
golangci-lint run                               0 issues   (see note in Scope findings)
gosec ./...                                     0 issues (35 files, 32 nosec)
bash scripts/check-lessons.sh                   0
bash scripts/check-actions-pinned.sh            0
bash scripts/check-workflow-permissions.sh      0
grep -rnE -- '--auth-key[ =]' docs README.md .env.example | grep -v auth-key-file
  -> exactly 2 hits: guide-multi-device-operations.md:67 (POSIX), :75 (PowerShell)
/usr/local-equivalent build: /tmp/ts-bridge-review
  init --auth-key-file /tmp/k --target 100.0.0.1:22  -> unknown flag: --auth-key-file (exit 1)
  init --help                                        -> recommends --auth-key-file (see F1)
  discover --help                                    -> --auth-key with no warning (see F2)
```

Mutation battery (each edit applied to `cmd/cli/init.go`, test run, then `git checkout --`):

| Mutation | Result |
|---|---|
| drop `--auth-key-file` from `init` Long | FAIL `TestInit_SecurityGuidance` (init_test.go:266) |
| drop "child processes" from `init` Long | FAIL `TestInit_SecurityGuidance` (init_test.go:269) |
| drop "visible in process list" from `--auth-key` usage | FAIL `TestInit_SecurityGuidance` (init_test.go:276) — exactly the line `verification.md` claims |
| drop the note from `buildEnvConfigContent` (env format) | FAIL `TestWriteEnvConfig_CreatesFullConfig` (init_test.go:259) |
| drop the note from `buildEnvContent` (YAML-mode `.env` sidecar) | **suite green** — unpinned (F3) |
| drop the whole `printNextSteps` security tip | **suite green** (`go test ./cmd/cli/`) — unpinned (F3) |

### Spec and task alignment

- **AC1 — met.** `README.md:52` names both exposures (`--auth-key` in the process list, keys in
  env/`.env` readable by child processes) and recommends `--auth-key-file`; `README.md:187` marks
  `TS_AUTHKEY` required "unless `--auth-key-file` is used", which matches the code path at
  `cmd/cli/connect.go:94-99` (file key lands in `flags.AuthKey`, and `applyFlags` writes it after
  `applyEnv`). `.env.example:7-9` carries the child-process note and the `0600` recommendation.
- **AC2 — met, with one wording gap (F1).** `init` Long carries the child-process line and the
  key-file line, `printNextSteps` prints the tip, and the flag usage carries the process-list
  warning; the three Long/usage strings are pinned by mutation. The examples block still shows the
  leaking form without an inline note, and the security note recommends a flag `init` does not
  register.
- **AC3 — met within its stated scope.** No `connect` example under `docs/` uses inline
  `--auth-key`; the only two survivors are the unattended `init` calls, both under the
  process-table warning, which is exactly what the rescoped AC3 claims. The rescope's three
  load-bearing facts check out: `docs/runbooks/guide-deployment-linux.md` has no CLI auth-key
  example (it authenticates through a `chmod 600` `EnvironmentFile`), there is no `TS_AUTHKEY_FILE`
  anywhere in the tree, and `scripts/host/ts-bridge.service` really was deleted in `e2cb699` (#156).
  `#306` and `#307` are both OPEN, so the deferrals are tracked, not verbal.
- **AC4 — met.** Full `-race` suite, lint and gosec green, reproduced above.
- **Round-1 dispositions hold.** The Major (AC3 vs. the Linux runbook) was resolved by rewriting
  the contract to name the exclusion rather than by leaving code to contradict it; the Minor on
  `TestInit_SecurityGuidance` was applied and I reproduced its mutation; the `Get-Content` Minor was
  declined for a reason I could not refute — `init` reads the key only from `--auth-key`
  (`cmd/cli/init.go:166`), so `TS_AUTHKEY=… ts-bridge init --target …` is ignored and prompts
  (`read auth key: inappropriate ioctl for device`, exit 1, measured), leaving the command line as
  the only unattended route until #306.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | Docs / help text | `ts-bridge init --help` security notes say "consider using `--auth-key-file /path/to/key`", but `init` registers no such flag, and the flag list printed directly below omits it. Following the recommendation as written exits 1: `ts-bridge init --auth-key-file /tmp/k --target 100.0.0.1:22` → `unknown flag: --auth-key-file` (exit 1). This is the same trap the same change documented for the runbook (`lesson-030`, `#306`), and the site page for this command states the opposite ("`init` has no `--auth-key-file`"). The examples block also still shows `--auth-key tskey-auth-xxx` three times without an inline warning. Mitigating: `#306` is the ticket that makes the line true, and `printNextSteps` already prints the correct `ts-bridge connect --auth-key-file …`. | `/tmp/ts-bridge-review init --help`; `init --auth-key-file …` exit 1; `site/src/content/docs/cli-reference.md:102-103` | UNTESTED — `TestInit_SecurityGuidance` pins only the substring `--auth-key-file` in `cmd.Long` (init_test.go:265), so it passes both before and after any rewording | code (one line in `cmd/cli/init.go` Long: name `connect`, or state that `init` has no key-file flag) |
| Minor | REAL | Coverage of entry points | `discover` is a third entry point that takes an inline key (`cmd/cli/discover.go:53`) and it got neither half of this hardening: its usage string has no process-list warning (unlike `connect.go:43` and `init.go:58`), it has no `--auth-key-file`, and it is absent from `site/src/content/docs/cli-reference.md`. The proposal's Out of Scope enumerates `init` (#306) and Linux (#307) but never mentions `discover`, and no open issue tracks it (only #306/#307/#343/#186 match "auth-key"). Nothing in `docs/` documents a `discover --auth-key` example, so AC3 is not violated — the gap is the unstated, untracked surface. | `/tmp/ts-bridge-review discover --help` → `--auth-key KEY  Tailscale auth key (overrides TS_AUTHKEY)`; `gh issue list --search auth-key` | UNTESTED — no test asserts any flag usage text other than `init`'s (grep of `*_test.go`: only `cmd/cli/init_test.go:271-276`) | code + spec (record it as a named exception alongside #306) |
| Minor | REAL | Tests | The proposal's "What" item 4 claims tests verify "help text, **next-steps output**, and error messages", and `verification.md` lists the mutation battery as the evidence. Two strings added by this change are pinned by nothing: the `printNextSteps` security tip and the YAML-mode `.env` sidecar note in `buildEnvContent`. Removing either leaves the full `cmd/cli` suite green, and the bats case that covers the YAML path asserts only that the key lands in `.env`, not the note. | Mutation rows 5-6 above; `cmd/cli/init.go:556`, `cmd/cli/init.go:663-665`; `scripts/tests/smoke.bats:199-211` | UNTESTED — no named test covers either string | tests (`TestInit_SecurityGuidance` or a captured-stdout subtest; one `assert_contains` in the existing bats YAML case) |
| Minor | THEORETICAL | Verification automation | AC3's invariant is enforced by a manual grep recorded in `verification.md`, not by anything that runs. The repo already wires three `scripts/check-*.sh` guards into the Repo hygiene job, and the guidance in this area has drifted twice before (#310 recovered the #298 remainder; #339 fixed the site pages that still taught the inline form), so a fourth drift would be caught only by whoever remembers to re-run the grep. The invariant holds today — I re-ran the enumeration and got the two documented hits. | `grep -rln 'auth-key' scripts/ .github/workflows/` → no hit; `.github/workflows/repo-hygiene.yml` runs only the lessons/actions/permissions guards | UNTESTED (no guard exists to name) | scripts + CI (a `check-doc-authkey.sh` twin of the existing guards), or a Go test over `docs/**` |
| Minor | REAL | Adjacent docs defect | `docs/runbooks/guide-deployment-linux.md` §5 still instructs `sudo cp scripts/host/ts-bridge.service /etc/systemd/system/`, but that path was deleted in #156 — the runbook cannot be followed end to end. The proposal cites that same deletion as its reason for deferring the Linux conversion, so the citation is accurate; the dangling step itself is pre-existing and owned by the open `#307`. Surfaced here because this change declared the file out of scope and a reader will otherwise assume the file is sound. | `git log --diff-filter=D --name-only -- '*ts-bridge.service*'` → `e2cb699` (#156); `#307` OPEN | UNTESTED (no test executes runbook steps) | docs (carry it in #307; no change needed for this spec to archive) |
| Question | — | Spec artifacts | This spec has no `features.json` while both newer specs (CI-314, CI-322) do, and `features.json` is one of the three contract files the archive gate measures staleness against. Its absence does **not** block the gate — `dotf`'s staleness check diffs and stats only the three named paths, and a path that does not exist contributes nothing — but it also means AC1-AC3 have no machine-readable behaviour→verification mapping, which is how the two newer specs carry theirs. Decide deliberately: leave it, or backfill it and accept that a new contract file makes this review stale by construction and forces another round. | `ls specs/SEC-212-authkey-hardening/` (no `features.json`); `cli/internal/spec/review.go:24,175-208` (dotfiles) | UNTESTED (n/a) | spec (a decision, not a fix) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1-AC4 are all met and I reproduced each; negative-path gaps are the doc-pinning ones in F1-F3, and the one surviving inline example is labelled with the reason it survives. |
| Verification       | B | Evidence is reproducible — I rebuilt, re-linted and re-ran the suite, and 4 of the 6 mutation claims in `verification.md` reproduce exactly — but two newly added strings are unpinned and AC3's invariant has no automated guard. |
| Scope              | B | The SEC-212 slice matches the proposal file-for-file (plus the #347 rescope); the review range also contains four unrelated specs' work, which is attributable to the launcher's base, not to creep here. |
| Reliability        | B | Docs-only change with no runtime path altered; the guidance matches the binary's real flag surface except for F1, and error paths (`--auth-key-file` missing/loose perms) are unchanged and pre-existing with tests. |
| Maintainability    | B | New strings live in one place per surface, functions stay short and dead code is not introduced; the ambiguity in F1 and the unpinned strings are the only smells. |
| Handoff-readiness  | A | Contract rescued in-session (AC3 rewritten to match what the docs can show, with the reason), tasks annotated with the rescope, round-1 dispositions tabulated, lesson-030 recorded, and #306/#307 open and scoped. |

Aggregation: no D, no C, no Blocker, no REAL Major — every open finding is Minor (three REAL, one
THEORETICAL, one adjacent, one question). The mechanical rule therefore permits PASS; the verdict
below is the gaps variant because three of the five Minors are places where this change's own
evidence claims more than it pins.

### Verdict
PASS WITH GAPS

### Recommended next steps

None of these touch the contract set (`proposal.md`, `tasks.md`, `features.json`); all are
applicable in the verification window without invalidating this verdict. Each is for the
implementer to disposition — applied, ticketed, or declined with a reason — in `verification.md`,
which the staleness check excludes for exactly that purpose.

- **[Disposition required] F1 — reword one line in `cmd/cli/init.go`'s `Long`** so the security note
  names `connect` (or says outright that `init` has no key-file flag and to use the interactive
  prompt). This is the same one-line class of fix `#305` made to the runbook, and the check that
  found it is the project's own rule in `lesson-030`: build the binary, run the documented command,
  read the exit status. Declining is defensible if `#306` is considered the whole fix — say so.
- **[Disposition required] F3 — pin the two strings this change added but does not test**: the
  `printNextSteps` tip (a captured-stdout subtest, or extend `TestInit_SecurityGuidance`) and the
  `buildEnvContent` sidecar note (one `assert_contains` in the existing bats YAML case, which
  already reads the `.env` it writes).
- **[Ticket or decline] F2 — decide what `discover` is.** Either add the process-list warning to its
  usage string and a `--auth-key-file` variant under `#306`'s umbrella, or record in the spec that
  `discover`'s inline key is a known, accepted exception. Silence is the one option that leaves a
  third entry point on the path this spec exists to close.
- **[Ticket] F4 — if AC3's invariant is meant to hold, give it a guard** (`scripts/check-*.sh` run
  from the Repo hygiene job, twin of the three that are already there); otherwise state in the
  spec's follow-up that it is a review-time check by choice. Publishing site docs remain outside
  AC3's letter (`docs/` only) and are covered by the closed `#313`, whose pages already carry the
  warning labels.
- **[Decision] F5 — fold the dangling `cp scripts/host/ts-bridge.service` in the Linux runbook into
  `#307`** (already open) rather than leaving it as a runbook that cannot be followed.
- **[Decision, cost attached] F6 — `features.json`.** Leaving it costs nothing the archive gate
  checks; backfilling it adds a fourth contract file, which makes this review stale by construction
  and needs another round. Choose knowingly rather than by omission.

Note on reproducing the lint result: `golangci-lint run` first reported two `unused` issues whose
paths were `/home/manu/Projects/ts-bridge-wt-sec212/...` — a sibling worktree resolving through the
shared analysis cache because both trees share the module path. After `golangci-lint cache clean`
the run is 0 issues, matching `verification.md`. That is an environment artifact, not a defect in
this change; it is recorded so the next reader does not read the stale cache as a regression.
Two files outside this spec (`specs/CI-314-pr-agent-reviewer/review.md`,
`specs/CI-322-workflow-least-privilege/review.md`) changed during this session from other review
runs and were left untouched; the SEC-212 working tree is otherwise clean.
