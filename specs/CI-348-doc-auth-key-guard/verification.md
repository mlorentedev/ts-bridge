---
tags: [spec, verification, templates]
created: "2026-09-30"
---

# Verification - CI-348-doc-auth-key-guard

## Evidence

- [x] Criterion 1 -> `bash scripts/check-doc-authkey.sh` reports
      `check-doc-authkey: OK (71 files)`. The `prose-only` and `clean` fixtures prove security
      guidance and `--auth-key-file` examples do not produce false findings.
- [x] Criterion 2 -> `scripts/tests/test-doc-authkey.sh` covers eleven fixtures: clean,
      two prose-only forms, same-line connect, equals-form connect, POSIX multiline connect,
      two PowerShell continuation forms, warned init, unwarned init, and compound init/connect.
      The bash run is 11/11 green; the Repo hygiene job installs real zsh and runs the same
      matrix on every PR.
- [x] Criterion 3 -> `.github/workflows/repo-hygiene.yml` invokes the guard before installing
      zsh and invokes the fixture suite afterward. `actionlint -shellcheck=shellcheck` is clean,
      as are all local hygiene, build, test, vet, and lint gates listed below.

## Test status

- RED 1: `TDA_SHELLS=bash bash scripts/tests/test-doc-authkey.sh` before the guard existed ->
  exit 2, `guard not found`.
- RED 2: after adding a POSIX multiline fixture but before continuation tracking -> exit 1 for
  the suite because `connect-multiline` returned 0 instead of 1.
- RED 3: after adding a PowerShell multiline fixture but before backtick tracking -> exit 1 for
  the suite because `connect-powershell-multiline` returned 0 instead of 1.
- RED 4 (CodeRabbit review): three fixtures failed against commit `8b7eb4a` exactly as reported:
  `prose-command-mention` returned 1 instead of 0, while `init-then-connect` and
  `connect-split-value` returned 0 instead of 1.
- Targeted GREEN:
  `TDA_SHELLS=bash bash scripts/tests/test-doc-authkey.sh` -> 11/11 fixtures passed.
- Real tree: `bash scripts/check-doc-authkey.sh` ->
  `check-doc-authkey: OK (71 files)`.
- Existing hygiene guards:
  - `bash scripts/check-lessons.sh` -> `OK (33 lessons)`.
  - `bash scripts/check-actions-pinned.sh` -> `OK (9 workflow files)`.
  - `bash scripts/check-workflow-permissions.sh` ->
    `OK (9 workflows, 12 checkouts, 12 credential-less, 0 opted out with a reason)`.
- Existing hygiene suites:
  - `TWP_SHELLS=bash bash scripts/tests/test-workflow-permissions.sh` -> all 26 bash fixtures
    plus the missing-shell self-test passed.
  - `bash scripts/tests/test-check-review-published.sh` -> all 4 fixtures passed.
  - `bash scripts/tests/test-site-pr-workflow.sh` -> `OK`.
- Workflow and shell lint:
  `actionlint -shellcheck=shellcheck` and
  `shellcheck scripts/check-doc-authkey.sh scripts/tests/test-doc-authkey.sh` -> no findings.
- Go gates:
  - `go build ./...` -> exit 0, no output.
  - `go test ./...` -> all packages passed.
  - `go vet ./...` -> exit 0, no output.
  - `golangci-lint run` -> `0 issues`.
- Local zsh note: the installed Scoop `zsh.exe` shim hangs on `zsh --version` and fails before
  parsing `set -o pipefail`, so it cannot certify zsh locally. This is an environment defect,
  not treated as a green skip: the default local matrix returned non-zero. The workflow installs
  Ubuntu zsh explicitly, and initial PR CI is the zsh evidence.
- No regressions in existing test suite: yes.

## Decisions made during implementation

- `site/src/content/docs/` is deliberately excluded. Issue #343 has not selected a single-source
  strategy, and the site contains warned quick-try inline examples. Including it would require
  editing content touched by PR #398, contrary to this PR's guard-only atomic scope.
- The guard accepts an optional scan root so committed fixture trees exercise the production
  parser without temporary directories or mutating repository documentation.
- Command continuation state covers both POSIX backslashes and PowerShell backticks; both forms
  failed first as dedicated fixtures before their support was added.

## Review window (PR #400)

| Finding | Disposition | Evidence |
|---|---|---|
| CodeRabbit Major: prose containing separate `` `ts-bridge connect` `` and `` `--auth-key value` `` spans was treated as an executable example | **Applied.** Command detection now requires an executable-looking command segment at the start of a shell segment (or a whole inline-code span). | `prose-command-mention` failed before the fix and passes afterward. |
| CodeRabbit Major: one `init` occurrence classified an entire compound line, allowing a later inline-key `connect` | **Applied.** Logical commands are split on `;`, `&&`, and `||`, then each `ts-bridge` invocation is classified independently. | `init-then-connect` failed before the fix and now reports the `connect` invocation. |
| CodeRabbit Security Architecture Low: a PowerShell backtick between `--auth-key` and its value bypassed the physical-line matcher | **Applied as tightly coupled.** Continuation lines are normalized into one logical command before argument matching. | `connect-split-value` failed before the fix and now reports the inline key. |

## Review round 1 (`agy/gemini-3.1-pro-high`, FAIL at `177355c`)

`review.md` records two Blockers and one Major, all REAL and all reproduced as fixtures that
returned exit 0 against `177355c` before the fix (RED 5, below). All three are fixed in
`scripts/check-doc-authkey.sh`; no contract file (`proposal.md`, `tasks.md`, `features.json`)
changed, so the next round reviews the same contract.

| Finding | Disposition | Evidence |
|---|---|---|
| Blocker: `sudo`, `VAR=value` assignments and absolute paths bypass `is_command` | **Applied.** `strip_prefixes` removes the PowerShell call operator `&`, `sudo` (with its flags), `env` and leading assignments; the binary match accepts any path prefix (`/usr/local/bin/`, `C:\tools\`). `sudo -u <user>` keeps its argument and is not unwrapped: no doc uses it, and the guard prefers a narrow, explicit prefix list over guessing option arity. | `connect-sudo-prefix`, `connect-env-prefix`, `connect-absolute-path`, `connect-windows-path` |
| Blocker: a whole-line inline code span ending in a backtick was read as a PowerShell continuation | **Applied.** A continuation backtick must follow whitespace (PowerShell's own rule); a backtick glued to text closes a code span. List markers (`-`, `*`, `+`, `1.`) are stripped so a span inside a list item is checked too. | `connect-inline-span`, `connect-list-inline-span`; the two existing PowerShell continuation fixtures stay green |
| Major: a single pipe did not split segments | **Applied.** Segments split on `;`, `&&`, `||` and `|`. | `connect-piped`; `pipe-no-key` proves a safe pipe and a wrapped `--auth-key-file` example stay clean |

- RED 5: the eight new fixtures against `177355c` -> the seven unsafe ones returned exit 0
  instead of 1; `pipe-no-key` returned 0 as wanted.
- GREEN: `bash scripts/tests/test-doc-authkey.sh` -> 38/38 (19 fixtures x bash + zsh), also
  19/19 under `mawk` with `TDA_SHELLS=bash`.
- Real tree: `bash scripts/check-doc-authkey.sh` -> `check-doc-authkey: OK (73 files)`.
- `shellcheck scripts/check-doc-authkey.sh scripts/tests/test-doc-authkey.sh` -> no findings.

## Review round 2 (`agy/gemini-3.1-pro-high`, FAIL at `c5cceef`)

One REAL Blocker and two THEORETICAL Majors. Contract files unchanged again.

| Finding | Disposition | Evidence |
|---|---|---|
| Blocker (REAL): `ts-bridge \` split right after the binary drops the trailing space `is_command` required | **Applied.** The binary match ends at whitespace *or* end of segment. | `connect-split-after-binary` returned 0 against `c5cceef`, 1 now |
| Major (THEORETICAL): `VAR="a b" ts-bridge` is not unwrapped | **Applied.** Assignments accept bare, double-quoted and single-quoted values. | `connect-quoted-env`, `connect-single-quoted-env` |
| Major (THEORETICAL): runners (`go run`, `docker run`) bypass the guard | **Applied for `go run`**, which `docs/runbooks/guide-launcher-parity.md` names as the binary fallback: `go run ./cmd/ts-bridge connect …` now unwraps to a path-prefixed binary. **Declined for `docker run`**: the project ships a single binary and no container image (no `Dockerfile`, ADR-002), so no documented invocation exists to guard, and an image name need not contain `ts-bridge` at all. | `connect-go-run` |

- GREEN: `bash scripts/tests/test-doc-authkey.sh` -> 46/46 (23 fixtures x bash + zsh).
- Real tree: `check-doc-authkey: OK (73 files)`; `shellcheck` clean.

## Review round 3 (`agy/gemini-3.1-pro-high`, FAIL at `fe7f821`) and the redesign

Round 3 found two more REAL Blockers and a REAL Major of the same kind as rounds 1 and 2 (a
quoted executable path, a root `# ` prompt, a subshell), plus a case-sensitivity Major, a
`--verbose init` false positive and a command-substitution assignment. Three rounds of bypasses
of one matcher meant the approach was wrong, not the patches: enumerating invocations cannot close
the class. The guard now classifies by **region** (see the threat model added to `proposal.md`,
which moves the contract digest on purpose): every logical command in a fenced or indented code
block is checked with no command recognition at all, and in prose only an inline code span that
names `ts-bridge`. `init` keeps its warned exception only as the single key of an unsplittable
segment, so a second key in a subshell is a finding.

| Finding | Disposition | Evidence |
|---|---|---|
| Blocker: quoted executable paths and names | **Fixed by the redesign** | `connect-quoted-path` (exit 0 against `fe7f821`, 1 now) |
| Blocker: root `# ` and `% ` prompts | **Fixed by the redesign** | `connect-root-prompt` |
| Major: subshells and brace groups | **Fixed by the redesign**; two keys in one segment are a finding even under `init` | `connect-subshell`, `init-subshell-two-keys` |
| Major (THEORETICAL): `.EXE` | **Fixed by the redesign** | `connect-upper-exe` |
| Minor (THEORETICAL): `ts-bridge --verbose init` false positive | **Fixed by the redesign**: `init` is a token, not a position | `init-flag-before-subcommand` (exit 1 against `fe7f821`, 0 now) |
| Minor: `FOO=$(echo val) ts-bridge` | **Fixed by the redesign** | `connect-command-substitution` |

- `connect-indented-code` pins the indented-code region.
- GREEN: `bash scripts/tests/test-doc-authkey.sh` -> 62/62 (31 fixtures x bash + zsh); 31/31
  under `mawk` and under `busybox awk`.
- RED against `fe7f821`: six of the eight new fixtures fail as described; `connect-indented-code`
  and `init-subshell-two-keys` were already caught and pin behaviour that must not regress.
- Real tree: `check-doc-authkey: OK (73 files)`; `shellcheck` clean.

## Review round 4 (`agy/gemini-3.1-pro-high`, FAIL at `7603f59`) — fixes landed, round 5 pending

The session that ran rounds 1-4 set itself a stop condition (one round after the redesign) and
kept it: the spec stays `verifying`. The round-4 findings were cheap and are fixed with fixtures
that failed first, so **round 5 reviews a tree with no known open finding**. No round 5 ran here.

| Finding | Disposition | Evidence |
|---|---|---|
| Blocker (REAL, false positive): a nested list item indented 4 spaces is read as indented code | **Applied.** Indented code starts only after a blank line outside a list; an indented line inside a list is the item continuation. | `nested-list-mention` exited 1 against `7603f59`, 0 now; `connect-indented-code` still fails as it must |
| Major (THEORETICAL): `VAR=" init " ts-bridge connect --auth-key …` gets the `init` exception | **Applied.** `init` counts only as a bare word; quoted strings, `$(…)` and backtick substitutions are removed first. | `init-word-in-quotes` |
| Minor (THEORETICAL): a ```` fence is closed by an inner ``` line | **Applied.** A fence closes only on a bare run of the same character at least as long as the opener (CommonMark). | `connect-long-fence` exited 0 against `7603f59`, 1 now |
| Minor (THEORETICAL): "What" still says "a `ts-bridge` command" | **Applied.** Reworded to the region rule, pointing at the threat model. | `proposal.md` |

- GREEN: 68/68 (34 fixtures x bash + zsh); 34/34 under `busybox awk`; real tree `OK (73 files)`;
  `shellcheck` clean.
- PR #421 PR-Agent (REAL, applied): a command left open by a dangling continuation was dropped when its fence closed, and an indented code block inside a list item was read as prose. The open command is now scanned before any fence transition, and inside a list only a nested list item stays prose. Fixtures `connect-fence-dangling-continuation`, `connect-list-indented-code` (exit 0 against `c4c7c5f`, 1 now); 36 fixtures green under bash and zsh, `mawk`, `busybox awk`; `nested-list-mention` still 0.
- **Next:** `dotf spec review CI-348-doc-auth-key-guard` (any live pool member; `deepseek` needs
  `dotf secrets unlock`), then archive on PASS / PASS-WITH-GAPS.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? no: lesson 032 already records that invariant
      checks must enumerate the full class and fail under mutation.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: this automates an existing
      SEC-212 documentation contract without changing architecture.
- [x] New pattern candidate for `00_meta/patterns/`? no: the existing guard-plus-fixture house
      pattern was reused without establishing a new cross-project method.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CI-348-doc-auth-key-guard/` -> `specs/archive/CI-348-doc-auth-key-guard/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
