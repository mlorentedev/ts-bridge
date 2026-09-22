---
id: "CI-322-workflow-least-privilege"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-09-21"
issue: "mlorentedev/ts-bridge#322"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, ci, security, github-actions, least-privilege]
template_version: "1.0"
---

# CI-322: least-privilege workflow permissions and credential-free checkouts

## Why

Every workflow in this repo that does not declare `permissions:` receives GitHub's repository
default — read-write on every scope — and every `actions/checkout` leaves the run token in
`.git/config` inside the workspace by default. So a job whose only job is to compile tests holds
a credential that can rewrite the repository, readable by every later step (a linter, a build, a
test that shells out). Nothing in this repo pushes from a workflow, which means the grant is paid
for and never spent: the whole exposure buys zero capability. `repo-hygiene.yml` already proves
the intended posture (`permissions: contents: read` + `persist-credentials: false`), which is why
this is a widening of one file into eight rather than a new invention — and why it needs a guard,
because a posture held by one example file decays back to the default on the next copied workflow.

## What

After this PR, an unauthorized grant is a **build failure** rather than a default:

- All eight workflows declare a top-level `permissions:` key, and the two that lacked one
  (`ci.yml`, `add-to-project.yml`) declare the minimum their steps actually use
  (`contents: read`, and `{}` for a job that authenticates with a PAT input and reads no repo file).
- All ten `actions/checkout` steps run with `persist-credentials: false`.
- `scripts/check-workflow-permissions.sh` enforces both rules and prints what it found, naming
  file and line. A checkout may opt out only with an inline `# persist-credentials-ok: <reason>`
  trailer, and the guard prints every opt-out whether or not the run is otherwise clean — an
  exemption that widens silently is the failure mode this whole change exists to prevent.
- `permissions: write-all` (at any level, and the `contents: write-all` form) is refused outright:
  it reads like a hardening declaration and grants the same ambient write scope as saying nothing.
- `scripts/tests/test-workflow-permissions.sh` proves the guard still refuses each way the
  posture can widen, under **both bash and zsh**.
- Both scripts run in the `Repo hygiene` job, so every PR and every push to `master` is gated.

## Out of scope

- **The identity model.** Release automation and the bitácora workflows still authenticate as one
  human user through a PAT, which is the root cause of #333 (a dead credential broke the release
  train silently, and the shared secondary rate limit is a second, independent fault). Moving them
  to a GitHub App is a separate decision with a wider blast radius than a permissions key.
- **Job-level permission narrowing.** The top-level key is the declared floor; per-job overrides
  remain allowed and are still checked for `write-all`. Auditing whether `pr-agent.yml` needs
  `issues: write` on every job is not done here.
- **Secret rotation and its runbook** — #333 option D.
- **Any Go code.** No production source, no CLI surface, no dependency change.

## Risks / open questions

- **Under-granting breaks CI.** `ci.yml` is the required status check, so a wrong permission key
  reds every PR in the repo. Mitigated by reading each job's steps for what it actually calls
  (nothing pushes, nothing comments) rather than by guessing, and by the CI run on this very PR
  being the proof.
- **Does any checkout legitimately need credentials?** Not today: the only writer to git from CI
  is release-please, and it is an action that authenticates with `RELEASE_PLEASE_PAT` as an input,
  not with the persisted git credential. If a future job must push, the named opt-out exists so the
  exception is visible in the log instead of becoming a deleted guard.
- **`persist-credentials: false` and `git rev-parse`** — `release.yml`'s `build-release` job runs
  `git rev-parse --short HEAD` after checkout. Read-only git operations do not need a stored
  credential; verified locally against a checkout built the same way.
- **Guard correctness beats guard cleverness.** The parser is indentation-based awk, not a YAML
  library: no new dependency (a zero-dep design goal here) and it runs anywhere CI runs. The cost
  is that a checkout step written on one line (`- {uses: ...}`) would be missed; the fixture suite
  is where that trade-off is visible, not in a comment nobody reads.

## Acceptance criteria

- [ ] Every workflow under `.github/workflows/` declares a top-level `permissions:` key, and none declares `write-all`.
- [ ] Every `actions/checkout` step in every workflow either sets `persist-credentials: false` or carries an inline opt-out naming a reason, and the guard reports the counts.
- [ ] The guard refuses each widening (missing key, persisted token, explicit `true`, `write-all`), passes the clean case, and behaves identically under bash and zsh.
- [ ] The guard and its test suite run in the `Repo hygiene` job, so the posture cannot decay on a later PR.
- [ ] No regression in the existing suite: `go build ./...`, `go vet ./...`, `go test ./...`, and the two pre-existing hygiene guards stay green.

## References

- Bitácora board: issue #322 (see `issue:` frontmatter) — picked up via self-assign so the board flips to In Progress.
- Related issues: #333 (one human PAT identity behind CI automation — the root cause this deliberately leaves alone), #335 (a `CODECOV_TOKEN` reference to a secret the repo does not have).
- Related pattern: `00_meta/patterns/pattern-security.md` (least privilege), `pattern-clean-as-you-go` (the guard, not just the fix).
- Existing precedent in-repo: `.github/workflows/repo-hygiene.yml` (the only hardened workflow before this change), `scripts/check-lessons.sh` / `scripts/check-actions-pinned.sh` (house style for guards that need no toolchain).
