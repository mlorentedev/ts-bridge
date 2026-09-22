---
tags: [spec, verification, ci, least-privilege]
created: "2026-09-21"
---

# Verification - CI-322-workflow-least-privilege

## Evidence

Branch `ci/least-privilege-workflow-permissions`, worktree `../ts-bridge-wt-ci-perms`, base
`master@3e71a7e`. Every command below was executed in this session; the output is pasted, not paraphrased.

- [x] **AC1 — every workflow declares a top-level `permissions:` key, none declares `write-all`**
  -> `bash scripts/check-workflow-permissions.sh` → `OK (8 workflows, 10 checkouts, 10 credential-less, 0 opted out with a reason)`, exit 0.
  Cross-checked with a YAML parser (not the guard's indentation logic), which printed the grant each
  workflow now carries and shows no `ABSENT`:

  ```
  add-to-project.yml   permissions={}
  bitacora-status.yml  permissions={}
  ci.yml               permissions={'contents': 'read'}
  mutation.yml         permissions={'contents': 'read'}
  pages.yml            permissions={'contents': 'read', 'pages': 'write', 'id-token': 'write'}
  pr-agent.yml         permissions={'contents': 'read', 'pull-requests': 'write', 'issues': 'write'}
  release.yml          permissions={'contents': 'write', 'pull-requests': 'write'}
  repo-hygiene.yml     permissions={'contents': 'read'}
  ```

  The two grants that stay write-bearing (`release.yml`, `pr-agent.yml`) were already declared
  before this change and are the two jobs that genuinely write: release-please opens the release
  PR/tag, pr-agent posts the review. They were not narrowed, because narrowing them is a different
  claim than the one this PR makes.

- [x] **AC2 — every `actions/checkout` runs credential-less**
  -> counted with independent tooling so a blind spot in the guard cannot certify itself:
  `checkouts=10  persisted_false=10` (equal). Nine of the ten are this PR's edits; the tenth was
  already hardened in `repo-hygiene.yml` and the scripted edit was verified not to double-patch it.

- [x] **AC3 — the guard refuses each widening, in both shells**
  -> `bash scripts/tests/test-workflow-permissions.sh`:

  ```
  ok [bash] clean -> exit 0, mentions "OK"          ok [zsh] clean -> exit 0, mentions "OK"
  ok [bash] missing-key -> exit 1, "no-perms"       ok [zsh] missing-key -> exit 1, "no-perms"
  ok [bash] persists-token -> exit 1, "no-except"   ok [zsh] persists-token -> exit 1, "no-except"
  ok [bash] explicit-true -> exit 1, "enabled"      ok [zsh] explicit-true -> exit 1, "enabled"
  ok [bash] write-all -> exit 1, "write-all"        ok [zsh] write-all -> exit 1, "write-all"
  ok [bash] named-opt-out -> exit 0, "opt-out"      ok [zsh] named-opt-out -> exit 0, "opt-out"
  ```

- [x] **AC4 — enforced on every PR and push** -> `repo-hygiene.yml` lines 27 and 29 run the guard and
  its test suite; the job triggers on `pull_request` and on `push: branches: [master]`.

- [x] **AC5 — no regression** -> `go build ./...` (silent), `go vet ./...` (silent), `go test ./...`
  all packages `ok`; `check-lessons.sh` → `OK (30 lessons)`; `check-actions-pinned.sh` →
  `OK (8 workflow files)`; `shellcheck -s bash` clean on both new scripts.

## Test status

- Guard behavior: 6 fixtures × 2 shells = 12 assertions, all green (AC3 above).
- Go suite: unchanged by this PR, run anyway because `ci.yml` itself changed — `go test ./...` green,
  `cmd/cli` 41.4 %, `internal/config` 92.2 % (measured before the branch, no coverage surface touched here).
- RED observed before GREEN: run against unmodified `master`, the guard exited 1 with 11 findings
  (9 `no-except` across `ci.yml` ×6, `mutation.yml`, `pages.yml`, `release.yml`; 2 `no-perms` for
  `ci.yml` and `add-to-project.yml`). That count matches the manual inventory taken before the
  script existed, which is the check that the parser is looking at the same tree a human read.
- Manual smoke: `git clone --depth 1 --no-local file://…` then
  `git -c credential.helper= rev-parse --short HEAD` → `3e71a7e`, proving the risk named in
  `proposal.md` (does `release.yml`'s `build-release` need the persisted credential for its
  `git rev-parse --short HEAD`?) is not real: rev-parse reads a local ref and never touches the remote.
- CI on this PR is the remaining evidence: `Repo hygiene` and `CI` must go green with the new keys in
  place, which is the only way to falsify the under-granting risk at runtime.

## Review window (PR #336)

PR-Agent ran on this PR (the first repository-side firing of the reviewer wired by #314) and its
`## PR Reviewer Guide` is dispositioned here rather than in chat:

| Finding | Disposition | Evidence |
|---|---|---|
| **Fragile YAML parsing** — a comment containing `: write-all` is flagged; flow-style `- {uses: …}` checkout is missed | **Applied (first half), accepted (second half).** The comment case was a real false red, reproduced immediately: a valid workflow with a commented-out checkout step exited 1 on line 8. Comment-only lines are now skipped and trailing comments stripped before matching; the opt-out trailer is read from the raw line because it *is* a comment. Two fixtures pin the negative. Flow-style steps stay a documented limitation — the honest fix is a real parser, which is #334 option B, not a second regex in a shell guard. | `bash scripts/check-workflow-permissions.sh /tmp/fp` → exit 1 before, exit 0 after |
| Ticket compliance: **#322 fully compliant** | No action — matches the acceptance criteria in `proposal.md`. | — |
| Ticket compliance: **#333 / #335 "not compliant"** | **Declined, with a reason rather than silence.** PR-Agent inferred obligations from the issues this PR's body *names as out of scope*. Neither is claimable here: the identity model (#333) and the coverage upload (#335) are separate changes with their own work-gates, and satisfying them would break the atomic-PR cap this PR is already declaring a breakdown to respect. | Out-of-scope section of the PR body |
| "PR contains tests", "No security concerns", effort 3/5 | Informational. | — |
| **CodeRabbit (Major, CWE-693) #1** — `permissions: "write-all"` is not matched, so the guard reports clean | **Applied.** Reproduced on a two-line fixture before touching the parser: the quoted declaration passed with `OK`. All three value matchers now share one optional-quote class, and `persist-credentials: "false"` is normalised so the fix cannot introduce the opposite error (a false red on a legitimate quoting). | `quoted-write-all` + `quoted-false` fixtures; `/tmp/qr` repro goes exit 0 → exit 1 |
| **CodeRabbit (Major, CWE-693) #2** — `uses: "actions/checkout@<sha>"` is not counted, so no `persist-credentials` is required | **Applied**, and the finding understated it: the step was not merely unenforced, it was **invisible** — the summary printed `0 checkouts` for a workflow containing one. An uncounted step cannot be reported missing. | `quoted-checkout` fixture; same repro, second line |
| CodeRabbit reached its own review here (unlike the `Review skipped` it returns on bot PRs) | Noted: this is the first CodeRabbit review of an agent-authored change in this repo, and both of its Majors were valid. Its walkthrough findings are dispositioned above, not summarised away. | — |

The two reviewers found different halves of the same defect class: **a text-matching guard enforces
the spellings it knows**. Comment-as-structure (PR-Agent) and quoting (CodeRabbit) are the same
bug wearing different clothes, and neither was visible from the eight cases that passed. After the
fixes: 11 fixtures × 2 shells = **22/22 green**, and the real tree still reports
`OK (8 workflows, 10 checkouts, 10 credential-less, 0 opted out with a reason)`.

## Decisions made during implementation

- **The guard, not just the fix.** Repairing eight files without a check leaves the posture held by
  one example file (`repo-hygiene.yml`), which is how it decayed in the first place. The guard turns
  a convention into a build failure.
- **awk, not a YAML library.** `check-lessons.sh` and `check-actions-pinned.sh` set the house rule:
  hygiene guards need no toolchain. A PyYAML dependency would make the guard un-runnable on a bare
  runner and add a dep to a zero-dep repo for a three-line question. Cost: a checkout step written
  in flow style (`- {uses: …}`) is invisible to the parser — a known, stated limitation, not a silent one.
- **`write-all` refused even though it is a "declared" permission.** A top-level
  `permissions: write-all` looks like the act of choosing and is the same ambient grant as the
  default. Detecting only the absent key would have let that pass.
- **Opt-outs are printed on clean runs too.** The first version hid them (they were only emitted
  inside the failing path), so an exemption that widened the posture looked like a pass — the exact
  failure this PR is about, reproduced inside its own guard. The fixture `named-opt-out` now pins
  the disclosure.
- **bash/zsh split divergence.** `set -- $sum` passes in bash and yields one argument in zsh; the
  guard reported "cannot parse guard output" on a *clean* repo. Fixed by splitting with `read` and
  validating the summary by `case` shape. Recorded here because the tempting fix was to make CI run
  only bash, which is the harness default this repo's own standards override.
- **Scope discipline.** The instinct was to also narrow `pr-agent.yml`'s `issues: write` and delete
  the dead `CODECOV_TOKEN` reference. Both are separate claims (#335 carries the second), and
  bundling them would make a security-Posture PR also a behavior-change PR. Left out, ticketed.
- **Process deviation, disclosed.** The guard and the workflow edits were written before
  `dotf spec init` ran, so `proposal.md` was filled after the code rather than before it. The gate
  itself held (#322 open, work-gate verified, TDD RED→GREEN inside the work); the paperwork order
  slipped. Nothing here argues that is fine — it is recorded because "we'll add the spec after
  merge" is the pattern the standing orders ban, and the fix is to do it in the same session.

## Promotion candidates

- [x] **Lesson for `docs/lessons/`?** Yes — three, all evidenced above rather than inferred:
  (1) a hygiene guard that hides its own exemptions is the failure mode it was written to prevent;
  (2) a text matcher enforces only the spellings it knows — comment-as-structure (PR-Agent) and
  quoted scalars (CodeRabbit) were the same hole twice, and no refusal-only fixture can see it;
  (3) `set -- $var` is not a portable splitter, so a guard whose contract is "bash and zsh" needs
  both shells in the test matrix. Written in this PR, not after merge.
- [ ] **ADR-worthy?** No. Least privilege in CI is covered by the language standards and the
  security pattern; declaring `permissions:` keys is not an architecture decision for a userspace
  TCP bridge. If the identity model changes (GitHub App, #333 option A), that is the ADR.
- [ ] **New pattern for `00_meta/patterns/`?** Candidate, not created here: "a one-time security fix
  must ship with the check that keeps it fixed" already recurs across repos (dotfiles, kubelab), so
  it belongs to the curator's recurrence path rather than to this PR's diff.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CI-322-workflow-least-privilege/` -> `specs/archive/CI-322-workflow-least-privilege/`
- [ ] Bitácora board ticket moved to Done / issue #322 closed with the PR link (`Closes #322`)
- [ ] Independent adversarial review recorded (`review.md`) by a model in `harness/reviewer-pool.json` — the implementer cannot sign it
- [ ] Promotions above executed (the two lessons)
