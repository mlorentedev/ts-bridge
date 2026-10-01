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
