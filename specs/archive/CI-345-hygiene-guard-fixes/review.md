---
spec: "CI-345-hygiene-guard-fixes"
verdict: "PASS"
reviewed_sha: "b9d37ff75e3ed6c645d5529ed2db808b955e18c7"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-01"
---

## Adversarial review

**Scope**: CI-345-hygiene-guard-fixes — the spec's own change, PR #359 (merge commit
`4ec249922e3bf02edb1ac1885c8527d70787f639`, first parent `e2ae3fa`, which is the `base_sha`
the launcher recorded in `review-request.json`).
**Sources**: `specs/CI-345-hygiene-guard-fixes/{proposal,tasks,verification}.md`,
`features.json`, `review-request.json`; `git diff e2ae3fa 4ec2499`;
`.github/workflows/{pr-agent,release,repo-hygiene}.yml`,
`scripts/check-review-published.sh`, `scripts/tests/test-check-review-published.sh`,
`harness/reviewer-pool.json`, `harness/review-attestation.json`, at `HEAD b9d37ff`.
Reviewed by pool primary `nan/deepseek-v4-flash` (launched 2026-10-01T04:24Z), never the
implementer. The full range `e2ae3fa..b9d37ff` additionally carries five unrelated merges
(#393–#398); those were read only where this spec's artifacts make claims about them (the
workflow counts) and are not part of the reviewed change.

### Spec and task alignment

- Every acceptance criterion was re-executed here, not read (results under "Reproductions"
  below). AC1 → `scripts/tests/test-check-review-published.sh` 4/4 green; AC2 →
  `actionlint -shellcheck=shellcheck` exit 0 at HEAD and exit 1 with `SC2035` on the pre-#359
  tree; AC3 → `go build ./...`, `go vet ./...`, `check-lessons.sh` (33 lessons),
  `check-actions-pinned.sh` (9 workflow files), `check-workflow-permissions.sh` (9 workflows,
  12 checkouts, 12 credential-less) all green.
- Each task claimed `[x]` in `tasks.md` is backed by diff evidence: the extracted guard, the
  four-fixture suite, the read-only checkout, the `release.yml --` fix, the actionlint step,
  and the post-CodeRabbit `ref: default_branch` pin are all present in the diff.
- The three `contract_digests` the launcher recorded were reproduced independently from disk:
  `proposal.md` and `tasks.md` match under the launcher's normalisation (line endings,
  lifecycle `status:`, checkbox ticks), and `features.json` matches through the
  unparseable-input byte fallback documented in Finding 4. The verdict therefore describes
  the contract as it stands, not a neighbouring revision.
- `verification.md`'s counts ("8 workflow files", "11 checkouts") were correct at #359 and
  have since drifted to 9 / 12 with `.github/workflows/site-pr.yml` (#381, after the merge);
  re-measured today, the guards report the larger numbers and stay green. Drift explained,
  not a defect.
- `test-workflow-permissions.sh` exits 1 on this Windows machine because zsh is absent, which
  is what `verification.md` discloses; the bash half is 26/26 green and the suite's own
  `missing-shell` case passes. No false green.
- No `[AGENT-DRAFT]` or `[AGENT-SUGGESTION]` remains in any spec artifact.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | Maintainability / doc drift | `.github/workflows/pr-agent.yml` now holds two comments that contradict each other. The registry comment says the marker comes "never from this job's own checkout, which is the PR head" (lines 195-196), but the same PR pinned that checkout to the default branch; the new checkout comment (lines 103-110) says the opposite and is the correct one. A security rationale that misstates where the trusted copy comes from is the kind of drift this PR fixed elsewhere. | `sed -n '190,200p' .github/workflows/pr-agent.yml` vs `sed -n '96,110p' ...`; the `ref: ${{ github.event.repository.default_branch }}` line | UNTESTED (prose) | code (comment) |
| Minor | REAL | Spec accuracy | `proposal.md`'s justification for the actionlint install misstates the in-repo precedent: "consistent with `golangci-lint`'s and `gosec`'s existing `go install ...@version` / `@latest` pattern in `ci.yml`". Measured: `ci.yml` runs golangci-lint as a SHA-pinned action (`golangci/golangci-lint-action@ba0d7d2e… # v9`), not `go install`; the `@latest` half is gosec (`ci.yml:180`), and the `@version` form appears in `mutation.yml:41` (`gremlins@v0.6.0`), not in `ci.yml`. The decision (tag-pinned `actionlint@v1.7.12`, checksum-verified by the Go module proxy) is defensible and better than `@latest`; only the stated precedent is wrong, and it archives with the spec. | `grep -n "golangci\|gosec\|go install" .github/workflows/ci.yml .github/workflows/mutation.yml` | UNTESTED (prose) | spec (contract set — recorded here, **not** edited) |
| Minor | REAL | Verification / tests | The guard's pagination handling is asserted only in prose. Both the script header ("`jq -s` slurps them into one array of arrays so the count spans every page") and the test header (which explains the single-page fixture shape) claim it; no fixture exercises more than one page. Mutation proof: replacing `.[][]` with `.[0][]` (first page only) leaves all four fixtures green while the guard then reports "published no review" for a marker on page 2 — the wrong-remedy red this spec exists to remove. The shipped code is correct (two-page fixture → exit 0), and the repo's comment volume is far below the 30-per-page boundary today (busiest recent PR: 9 comments), which is why this is Minor rather than Major. It becomes Major if the pr-agent guard ever becomes a required check or comment volume approaches one page. | Reproduced: two-page fixture against the real guard → `review published`, exit 0; against the `.[0][]` mutant → false red, exit 1; the mutant passes the whole suite. Comment counts via `gh api .../issues/<n>/comments --paginate --jq length` (max 9, #360) | UNTESTED (`scripts/tests/test-check-review-published.sh` has no multi-page case) | tests |
| Minor | REAL | Spec artifact | `features.json` diverges in shape from the spec scaffold: it is an object (`{"feature_id": …, "acceptance_criteria": […]}`) while `cli/internal/spec/templates/features.json` and every sibling spec (`CI-344`, `UX-355`, `CLI-306`) are arrays of `{id, behavior, verification, state, evidence}`. Consequence, demonstrated: `ContractDigests`' `normaliseFeatures` unmarshals `[]map[string]any`, so an object hits the "unparseable input digests by its bytes" branch — which is also why the recorded digest equals the raw byte digest. The harness-owned `state`/`evidence` fields are therefore not folded for this spec (they are for its siblings), so a later edit to those progress fields would read as content drift and let `dotf spec archive` refuse as stale — the failure the fold exists to prevent. No functional consumer reads the file today, so the harm is latent. | Go probe: CI-345 `NORMALISER FAILS -> byte fallback: json: cannot unmarshal object into Go value of type []map[string]interface {}`; CI-344/UX-355 parse (3 and 5 features). Recorded digest `59974f7f…` reproduced as sha256 of the LF-normalised bytes | UNTESTED | spec (contract set — recorded here, **not** edited) |
| Minor | THEORETICAL | Verification / CI scope | The new shellcheck pass lints workflow-embedded shell only. `scripts/*.sh` — including the guard script and suite this PR adds — are not shellchecked in CI, although `AGENTS.md` states "ShellCheck enforced" for `scripts/` and this PR is the one that introduces shellcheck. Latent rather than red: `shellcheck 0.11.0` reports nothing on either new script (or any `scripts/*.sh`). | `grep -rn shellcheck .github/workflows/` → only `repo-hygiene.yml`; `shellcheck scripts/check-review-published.sh scripts/tests/test-check-review-published.sh` → exit 0 | UNTESTED | code (CI step) |
| Question | SPECULATIVE | Resilience | `(.body // "")` narrows the abort to a `null` body. A non-string `body` still aborts the whole filter with a raw jq error and exit 5 — fail-closed, never a false green, but the same "bare stack trace instead of the guard's remedy" shape AC1 set out to remove. GitHub's issue-comment schema is string-or-null, so no occurrence is claimed. | `echo '[{"body":1}]' \| jq -s '…((.body // "") \| contains("m"))…'` → exit 5, `number (1) and string ("m") cannot have their containment checked`; same for an object body | UNTESTED | code (optional hardening, e.g. `tostring`) |

### Reproductions performed (independently, in this session)

1. **RED-GREEN for AC1.** Reverting the fix in a temp copy of the guard (`.body // ""` → `.body`)
   makes the suite fail on exactly the two null-body fixtures with
   `jq: error … null (null) and string ("## PR Reviewer Guide") cannot have their containment checked`
   (exit 5), while `clean-match` and `no-match` still pass. The two new cases are load-bearing,
   not decorative.
2. **RED for AC2.** `actionlint -shellcheck=shellcheck` on the pre-#359 workflow tree (extracted
   from `e2ae3fa`) reports `release.yml:49:9: shellcheck … SC2035 … Use ./*glob* or -- *glob*
   so names with dashes won't become options`, exit 1; on HEAD, 0 findings, exit 0. `release.yml`
   now carries the minimal `sha256sum -- *` correction.
3. **AC2 on master.** `repo-hygiene.yml` step-level results on post-merge runs
   (e.g. 36808014960, 36800827048): `Install actionlint` success,
   `Lint workflows and their shell (actionlint + shellcheck)` success. `hygiene` is in the
   branch's required checks (`test, lint, security, hygiene`, `strict: true`,
   `enforce_admins: true`), so the lint ride on a required job.
4. **Digest reproduction.** All three `contract_digests` in `review-request.json` reproduced
   from disk with the launcher's own normalisation (and, for `features.json`, its fallback
   branch) — the review is fresh and is about the current contract.
5. **Pins verified.** Both newly added `uses:` pins resolve to the tag they claim:
   `actions/checkout@3d3c42e5…` and `actions/setup-go@b7ad1dad…` are exactly the commits of
   their `v7` tags (`gh api repos/<owner>/<repo>/git/ref/tags/v7`).

### Hypotheses tested and refuted

- **The added checkout newly activates `.pr_agent.toml`.** Refuted: PR-Agent loads that file
  from the default branch regardless of a checkout (`pr-agent.yml:150`, measured upstream on
  2026-09-06 bootstrap PRs), and env wins over the file, so a workspace copy of the same
  master revision changes nothing. The new comment's "the only file this job's later steps
  need" is accurate.
- **The 5-minute `timeout-minutes` on the hygiene job is now a flakiness risk** (added
  `setup-go` + `go install`): refuted by measurement — 25 runs since the merge range from
  0.30 to 0.82 minutes per hygiene job, all successful.
- **`--paginate` multi-page handling is broken.** Refuted for the shipped code: a two-page
  fixture (two bare arrays, `--paginate`'s real stdout shape) yields `review published`, exit 0.
  The gap is the missing test, recorded as Finding 3.
- **The CodeRabbit-driven pin to `default_branch` loses coverage.** Checked: the guard is
  generic marker-matching and reads its marker from the same ref by API, so pinning the
  script's source to the default branch does not change the verdict it computes; it only
  removes a same-repo PR's ability to decide its own verdict with a write-capable token.
  Correct trade, and the residual cost (a PR that introduces the guard script cannot run it
  from master before merge) is already behind us and was masked on #359 by the PR-Agent
  15-minute cancellation, which `verification.md` discloses.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | All three criteria verified here, the null-body regression and its fail-closed control both covered, multi-page path proved correct, no defect observed in shipped behavior. |
| Verification       | A | Every claim in `verification.md` was re-executed with its RED reproduced; digests and action pins checked; two spec claims (workflow counts, `.pr_agent.toml`) explained against current reality. |
| Scope              | A | Diff matches the proposal exactly (3 workflows, 2 scripts, 4 spec files); the post-review checkout pin and the `release.yml` fix are disclosed in the artifacts; no creep, no Go touched. |
| Reliability        | B | Every mutation tried fails closed (jq abort, first-page-only, missing marker), but a non-string body still yields the bare-jq shape and two claims (pagination, script-absent-on-pinned-ref) have no covering test. |
| Maintainability    | B | Guard extracted along the house pattern, small functions, both new scripts shellcheck-clean; one rationale comment now contradicts the code in the same file. |
| Handoff-readiness  | B | Proposal, tasks and verification are complete and honest, promotion candidates declared; the lesson is deferred rather than written and `features.json` leaves a non-template shape for the next session. |

### Verdict
PASS

### Recommended next steps

Routed by set. The contract set (`proposal.md`, `tasks.md`, `features.json`) is closed by this
verdict: editing any of them would make `dotf spec archive` refuse this review as stale. Both
contract-set items below are Minor and are carried, not fixed, alongside the verdict.

- **tests (outside the contract set, safe to apply now):** add a `two-pages` case to
  `scripts/tests/test-check-review-published.sh` — the stub already `cat`s the fixture file, so
  concatenating two bare arrays reproduces `--paginate`'s real stdout. This is the named
  regression Finding 3 lacks.
- **code (comment, outside the contract set):** correct `.github/workflows/pr-agent.yml`
  lines 195-196 to stop describing the job's checkout as the PR head (Finding 1).
- **code (CI step, optional):** extend the actionlint step with `shellcheck scripts/*.sh` so
  the shell gate covers what `AGENTS.md` claims it covers (Finding 5); both new scripts already
  pass, so this adds a guard, not a fix.
- **ticket (bitácora):** file the two spec-artifact items that must not be fixed in place now —
  the `features.json` shape versus the scaffold (Finding 4) and `proposal.md`'s precedent
  sentence (Finding 2). Record both dispositions, with the corrected measurement for Finding 2,
  in `verification.md` (excluded from the staleness check).
- **verification.md:** record the dispositions of this review's findings, and answer the two
  "Promotion candidates" lines (`yes: <path>` or `no: <reason>`) — the archive pre-flight
  refuses without them, and they are the only archive blocker left.
- `dotf spec archive CI-345-hygiene-guard-fixes` is advisable once (a) the promotion-candidate
  lines are answered in `verification.md` and (b) the archived `proposal.md` status rewrite is
  the only contract-file change: the status line is folded by the digest normaliser, so it does
  not stale this review. Do not edit the contract set for the two Minor spec items.
