---
spec: "DEPS-312-go-127-tailscale"
verdict: "FAIL"
reviewed_sha: "aa3e8c6bc90b335b78608bec39fd674a5cfd95e8"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-27"
---

## Adversarial review

**Scope**: `DEPS-312-go-127-tailscale` — Go 1.27.1 + Tailscale v1.104 toolchain upgrade, module-derived CI toolchains, pinned linter, toolchain guard.
**Sources**: `specs/DEPS-312-go-127-tailscale/{proposal,tasks,verification,features}.md|json`, `specs/DEPS-312-go-127-tailscale/review-request.json` (base `f53847557c0b357998a16f55c622b02288d9a019` → reviewed `aa3e8c6bc90b335b78608bec39fd674a5cfd95e8`), spec commit `eab4bee` (#436), `go.mod`, `.github/workflows/{ci,release,mutation,repo-hygiene}.yml`, `scripts/tests/test-go-toolchain.sh`, `mise.toml`, `AGENTS.md`, `docs/lessons/lesson-037-2026-10-09.md`. Launcher-recorded contract digests re-checked with `dotf spec digest` — identical to `review-request.json` (contract unmodified since the request).

### Spec and task alignment

- **Contract integrity**: `dotf spec digest DEPS-312-go-127-tailscale` returns exactly the three `contract_digests` in `review-request.json`; no post-request edit to `proposal.md` / `tasks.md` / `features.json`.
- **Base resolution**: `f538475` is the parent of `eab4bee` (#436), so the spec's own diff is `git diff f538475 eab4bee` — 14 files, no application/protocol/CLI change, matching the proposal's scope statement. The `f538475...HEAD` range also contains 39 later merges that are not this change; they are treated as environment, not as scope creep.
- **AC1** — `go.mod` has `go 1.27.1`; `go mod tidy` is semantically clean (`git diff --exit-code -- go.mod go.sum` → 0, and `go mod tidy -diff`'s 524-line diff is CRLF churn only: 0 unique lines after stripping `\r`). Caveat: `tailscale.com` is `v1.104.1`, not the `v1.104.0` the criterion names — drifted by #438 after this spec landed (finding F3).
- **AC2** — `bash scripts/tests/test-go-toolchain.sh` → `OK (4 Go workflows)`; every `setup-go` step in `ci`, `release`, `mutation`, `repo-hygiene` uses `go-version-file: go.mod`; the pinned-1.26, wrong-step, missing-setup, and conflicting-pin fixtures are all rejected. The guard is wired into `Repo hygiene`, which runs on `pull_request`, so it does gate PRs (not only `master`). Enforcement has a demonstrated fail-open (F2).
- **AC3** — fresh evidence: CI run `40544475350` at the reviewed SHA is green on `lint`, `test`, `test-windows`, `test-macos`, `smoke`, `security` and all six `build-matrix` legs; locally `go build ./...` → 0, `go vet ./...` → 0, `go test -count=1 ./...` → 0 (13 packages ok, `HOME`/`USERPROFILE`/`APPDATA` redirected to a synthetic temp dir so no live profile was touched). `golangci-lint` was **not** executed locally: this workstation's endpoint protection flags it and the owner prohibits it, so the lint claim rests on the green CI `lint` job at the reviewed SHA. The criterion is met in substance but is still unchecked in `proposal.md`/`verification.md`, whose evidence cites dead head `b8ff388` (F5).
- **AC4** — `AGENTS.md` says `Go 1.27.1+`; the linter pin is single-sourced in `mise.toml` (`2.14.0`) and `ci.yml` (`golangci-lint-action@… version: v2.14.0`), which is correct under the post-#464 policy. The criterion's *machine-readable* verification no longer passes (F1).
- **Tasks** — all implementation boxes ticked and each maps to a real artifact except `[ ] PR opened`, which is stale (`#436` merged). `tasks.md`'s claim "every criterion has a non-vacuous `features.json` verification" is false at the reviewed SHA (F1, F4).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | verification contract | `features.json` f4 verification is unsatisfiable at the reviewed SHA: `grep -q 'golangci-lint@v2.14.0' AGENTS.md` exits 1. It was valid at `eab4bee`, then #464 (`331dec1`) moved the pin to `mise.toml` and made the guard *forbid* restating the version in `AGENTS.md` — so the spec's own predicate now contradicts the policy the same guard enforces. The harness can never set f4 to `passing`, and `features.json` is in the contract set. | Ran the f4 command verbatim: `grep1 OK` (`Go 1.27.1+`), `grep2 FAIL`. `AGENTS.md:48-50` points to `mise.toml` instead. | UNTESTED — nothing executes the `features.json` verifications in-repo | spec (`features.json`) → contract-set edit, re-review |
| Major | REAL | guard robustness | The toolchain guard fails **open** on a cosmetic YAML edit: job tracking keys on the exact line `^jobs:[[:space:]]*$`, so `jobs:   # job list` leaves `in_job` false forever, every job-internal check becomes inert, and a job that runs `go build`/`go test` with **no** `setup-go` is accepted. The `count > 0` floor cannot catch it because it is global across workflows, not per file. This defeats the "fail-closed toolchain policy" the proposal claims to retain. | Throwaway copy (`/tmp/tguard*`, tracked tree untouched): control = remove `setup-go` from `ci.yml` `test` job → `EXIT_CONTROL=1` (rejected); same edit plus `jobs:   # job list` → `test-go-toolchain: OK (4 Go workflows)`, `EXIT=0` (accepted). | UNTESTED — no fixture covers a commented `jobs:` key | code (`scripts/tests/test-go-toolchain.sh`) + tests (new fixture) |
| Minor | REAL | spec accuracy | `proposal.md` AC1 names Tailscale `v1.104.0`; `go.mod` requires `v1.104.1` since #438. The intent (1.104.x on Go 1.27.1) still holds, but the criterion is literally false at archive time. | `grep -n "^go \|tailscale.com v" go.mod`; `git log -S "tailscale.com v1.104.1" -- go.mod` → `06cfb87` (#438) | UNTESTED (textual criterion) | spec (`proposal.md`) → contract-set edit, re-review |
| Minor | REAL | verification contract | `features.json` f3 verification hardcodes `go run …golangci-lint@v2.14.0`, bypassing the mise pin and restating a version the post-#464 policy deliberately removed from operator docs; it is also unrunnable on this workstation (local lint prohibited) and slower/inconsistent with CI. | `features.json` f3 vs `AGENTS.md:48-50` and `mise.toml` | UNTESTED | spec (`features.json`) |
| Minor | REAL | handoff | `verification.md` criterion 3 is unchecked and cites PR #436 head `b8ff388`, a commit orphaned by the squash-merge; `tasks.md` `[ ] PR opened referencing this spec folder` is stale though #436 merged. A later reader cannot reproduce the cited evidence. | `git log --oneline -1 b8ff388` → unknown revision; #436 = `eab4bee` | UNTESTED | spec (`verification.md`, `tasks.md` — tick only; normalised out of the digest) |
| Minor | THEORETICAL | toolchain reproducibility | `go.mod`/`go.sum` are not covered by `.gitattributes` `eol` rules, so on this Windows checkout `go mod tidy -diff` exits 1 with a 524-line CRLF-only diff. A Windows-run harness recording f1 sees a red verification for a file state that git considers unchanged. | `tr -cd '\r' < go.sum | wc -c` → 262; normalised unique diff lines → 0; `git diff --exit-code -- go.mod go.sum` → 0 | UNTESTED | code (`.gitattributes`) |
| Minor | SPECULATIVE | guard coverage | The guard only recognises a whitelisted `go <subcommand>` at line level: `go get` (Go 1.24 `-tool` installs), wrapper scripts (`make test`, `./scripts/*.sh`) and Go run through other actions are invisible. Nothing validates a `toolchain`/`GOTOOLCHAIN` directive, although the proposal claims CI "rejects a future toolchain mismatch rather than silently downloading another compiler". | Code read of the `go[[:space:]]+(build|…|vet)` alternation and `check_linter`'s `$1 == "go"` filter; no reproduction | UNTESTED | code + tests (if promoted) |
| Minor | THEORETICAL | shell portability | `AGENTS.md` requires `scripts/*.sh` to run under bash **and** zsh; CI runs this guard under bash only, and zsh is absent from this workstation, so zsh parity is unverified. | `command -v zsh` → none; `repo-hygiene.yml` step runs `bash scripts/tests/test-go-toolchain.sh` | UNTESTED | tests (run under both shells, as `test-workflow-permissions.sh` already does) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | Change meets all four criteria in substance (build/vet/test/lint green at reviewed SHA), but AC1's text has drifted and AC2's enforcement has a demonstrated bypass. |
| Verification       | C | One of four `features.json` verifications fails outright; criterion 3's evidence cited a dead head and had to be re-produced by this review. |
| Scope              | A | `eab4bee` touches 14 files, all named by the proposal; no application, protocol or CLI change. |
| Reliability        | C | The guard silently stops checking on a cosmetic `jobs:` comment; a Windows-run tidy check is EOL-red without a real difference. |
| Maintainability    | B | Guard's awk fixture suite is readable and documented; the hardcoded `case v2.14.0) built_go=1.27` allowlist is fail-closed but forces a script edit on every bump (deliberate, noted). |
| Handoff-readiness  | C | Lesson 037 captured and the ADR correctly declined, but `features.json`, `verification.md` and `tasks.md` lag reality and one is grep-ably broken. |

### Verdict

FAIL — two **REAL Major** findings: an unsatisfiable verification in the contract set (F1) and a demonstrated fail-open in the guard that is the sole enforcement of AC2 (F2). Rubric path alone (C present, no D) would be PASS WITH GAPS; the findings axis escalates to FAIL. `specs/DEPS-312-go-127-tailscale/review.md` is the deliverable; archiving is **not** advisable in the current state.

### Recommended next steps

Contract-set items (each invalidates this verdict — fix, then re-run `dotf spec review DEPS-312-go-127-tailscale`, which is the mechanism, not an inconvenience):

1. **F1 — `features.json` f4**: replace the broken grep with a predicate that matches the adopted single-source policy, e.g. `bash scripts/tests/test-go-toolchain.sh` (covers Go floor + CI pin + mise pin + `AGENTS.md` policy in one command) or `grep -q 'Go 1.27.1+' AGENTS.md && grep -q '^mise install' AGENTS.md`. Confirm the command exits 0 before re-requesting review.
2. **F4 — `features.json` f3**: drop the hardcoded `@v2.14.0`; lint evidence belongs to CI, or reference the pinned local tool (`mise install && golangci-lint run`) without restating the version.
3. **F3 — `proposal.md` AC1**: restate as `Tailscale v1.104.x` (or `v1.104.1`) so the criterion is true at archive time; re-tick AC3 now that CI at this SHA is green.

Code + tests (also contract-adjacent only in that the guard is the const in `features.json`; the guard itself is not in the digest set):

4. **F2 — fix the fail-open and prove the fix**: accept a trailing comment on the `jobs:` key (or detect jobs structurally rather than by regex), and add two fixtures to `scripts/tests/test-go-toolchain.sh` — one asserting a `jobs:  # comment` workflow whose Go job lacks `setup-go` is **rejected**, one keeping the current control green. Until a fixture goes red on the unfixed guard, this Major stays open.
5. **F2 evidence to record** in `verification.md`: the exact mutation (comment the `jobs:` key, delete the `setup-go` step) and the guard's exit codes before/after.

Outside the contract set — no re-review needed, disposition in `verification.md` (applied / ticketed / declined with reason):

6. **F5** — replace the `b8ff388` citation with CI run `40544475350` at `aa3e8c6` and the local `go build`/`go vet`/`go test` results above; tick `tasks.md`'s PR box (checkbox ticks are normalised out of the digest).
7. **F6, F7, F8** — ticket rather than fix in scope: `.gitattributes` EOL rules for `go.mod`/`go.sum`; guard coverage for `go get`/wrapper scripts/`toolchain` directive; running the guard under zsh. Each is a real, small, separable defect — file it with root cause instead of leaving it in this table.

Review hygiene for the next round: the implementer cannot sign this spec's review (`AGENTS.md` → verification window; the archive gate refuses a self-signed pass), and the next `review-request.json` must be regenerated so its `contract_digests` cover the edited contract set.
