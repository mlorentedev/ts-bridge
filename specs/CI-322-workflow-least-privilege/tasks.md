---
tags: [spec, tasks, ci, least-privilege]
created: "2026-09-21"
---

# Tasks - CI-322-workflow-least-privilege

> TDD order. One task = one focused commit. Tick as you go.
>
> Deviation recorded up front rather than in conversation: the guard and the workflow edits were
> written before this folder existed (see `verification.md` → Process). The gate itself was honored
> — issue #322 open, work-gate verified by `dotf spec init` — and the RED→GREEN discipline was
> honored inside the work; what slipped was only the order of the paperwork, so it is fixed in the
> same session instead of after merge.

## Setup

- [x] Branch created from master: `ci/least-privilege-workflow-permissions`, in external sibling worktree `../ts-bridge-wt-ci-perms` (Standing Order #9 — never nested in the working tree)
- [x] `proposal.md` complete and every acceptance criteria testable
- [x] No open questions left in `proposal.md` "Risks / open questions" — each carries the mechanism that resolves it
- [x] Work-gate: `dotf spec init CI-322-workflow-least-privilege --issue 322` verified #322 open
- [x] Issue picked up: self-assigned #322, board flips to In Progress via `bitacora-status.yml` (observed green on the assign event)

## Implementation

- [x] [AC1][AC2] **RED first**: inventory what each job actually does (no push, no comment, no repo write) so the grant can be chosen rather than guessed; then run the new guard against unmodified `master` and record the findings — 11 across 5 workflows
- [x] [AC3] Write `scripts/check-workflow-permissions.sh`: top-level `permissions:` key required, `write-all` refused, every `actions/checkout` must set `persist-credentials: false` or name an inline opt-out; portable bash+zsh, BSD-find-safe, no new dependency
- [x] [AC3] Fix the guard's own first failure: it reported "cannot parse guard output" under zsh on a *clean* repo, because `set -- $sum` does not word-split there. Splitting moved to `read`; the case is now a named fixture
- [x] [AC2] Make the opt-out a **disclosure, not a finding**: the guard prints opt-outs even when the run is otherwise clean, so a widening exemption cannot arrive as a quiet pass
- [x] [AC1] Add the missing `permissions:` keys — `ci.yml` → `contents: read`, `add-to-project.yml` → `{}` (it authenticates through the `BITACORA_PAT` action input and reads no repo file), each with the reason in a comment
- [x] [AC2] Set `persist-credentials: false` on the nine unhardened `actions/checkout` steps (`ci.yml` x6, `mutation.yml`, `pages.yml`, `release.yml`), applied by script and reviewed as a diff so `repo-hygiene.yml`'s existing hardened step could not be double-patched
- [x] [AC3] Write `scripts/tests/test-workflow-permissions.sh`: six fixtures (`clean`, `missing-key`, `persists-token`, `explicit-true`, `write-all`, `named-opt-out`) each asserting exit code **and** the message, every fixture run under both shells
- [x] [AC4] Wire guard + test suite into the `Repo hygiene` job so the posture is enforced on every PR and push
- [x] Refactor for clarity: findings printed as `kind / file / line`, summary line separated from findings so the shell never parses prose

## Closing

- [x] Every acceptance criterion covered by at least one test or an executed command (mapping in `verification.md`)
- [x] Every acceptance criterion has a matching `features.json` entry with a non-vacuous verification command
- [x] Type checks pass — `go build ./...`, `go vet ./...` (no Go touched; run anyway because `ci.yml` itself changed)
- [x] Lint passes — `shellcheck -s bash` clean on both new scripts
- [x] All hygiene guards green — `check-lessons.sh`, `check-actions-pinned.sh`, `check-workflow-permissions.sh`
- [x] No unrelated changes in the diff
- [x] `verification.md` filled with executed evidence, not intentions
- [ ] PR opened referencing this spec folder (`Closes #322`)
- [ ] Reviewer output dispositioned and recorded under `## Review triage` on the PR
- [ ] Independent adversarial review before archive — the implementer cannot sign it (`harness/reviewer-pool.json` excludes Anthropic models for that reason)

## Machine-readable features

See `features.json` beside this file. AC5's cross-check deliberately does **not** call the guard:
it counts `checkout` steps and `persist-credentials: false` lines with independent tooling, so a
guard that quietly stops seeing checkouts cannot make its own criterion pass.
