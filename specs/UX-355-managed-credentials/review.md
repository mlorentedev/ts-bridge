---
spec: "UX-355-managed-credentials"
verdict: "FAIL"
reviewed_sha: "3213cec02b20818bb99778edbaf6519db096b298"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-01"
---

## Adversarial review

**Scope**: UX-355-managed-credentials (round 4) — whole spec-scoped change `8dfa4410a9bee7997fd72e0c614c34a3ff7e56bb...3213cec02b20818bb99778edbaf6519db096b298`, with emphasis on the round-3 remediation in `3213cec`.
**Sources**: `specs/UX-355-managed-credentials/{proposal,tasks,verification}.md` + `features.json` + `review-request.json`; diff `aad5fc9` (#388), `c51c845` (#389), `929a66a` (#392), `7cdcb41` (#393), `b9d37ff` (#398), `63a2939`, `f0ea65b`, `3213cec`. Round-3 findings were re-tested against `3213cec`, not accepted on the author's word.

### Spec and task alignment

- All eleven `tasks.md` boxes are `[x]`; no `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags remain in the spec folder (the only hit is the previous review's prose).
- **Round-3 Major 1 (browser/connect precedence) — resolved as a declared decision, and now coherent.** `proposal.md` AC3 was narrowed to state the ADR-014 browser boundary explicitly; `site/src/content/docs/configuration.md` adds a "Browser exception (ADR-014)" paragraph and the `TS_AUTHKEY` row now says browser ignores it; `warnIgnoredBrowserEnvCredential` emits a diagnostic on stderr; `TestBrowserManagedProfileCredentialOverridesEnvironmentWithWarning` pins managed-over-environment and the warning. The three artifacts no longer disagree. The round-3 open question is closed by the AC3 amendment, not left dangling.
- **Round-3 Major 2 (no rollback proof) — resolved.** `Store.rename` is now an injectable per-store seam and `TestStoreFailedAtomicReplacePreservesExistingCredential` asserts byte-identical preservation and no `.tmp` residue when the rename fails.
- **Round-3 Major 3 (f1 gate vacuous) — resolved.** f1 now selects eight named store/path tests. Reproduced the previous experiment: neutering `ValidateName` turns f1's own command **red** (`TestStoreValidatesNamesAndKeysWithoutLeakingValues/name=`, `name=two_words`, `name=.hidden`), where it was green before. Production code restored; `git status --porcelain` empty.
- **The same defect class survives in f4.** See finding 1 — f1 was fixed where it was reported, but f4 was never swept.
- `verification.md` is now accurate: the stale BUG-024 / BUG-023 blocker lines are corrected and the validation actually run is recorded.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | verification contract — f4 gate | `features.json` f4's behavior is "Connect and browser resolve profile credentials with **explicit-source precedence**", but its command `-run 'ProfileCredential\|BrowserCommand'` does not select the one test that pins connect's explicit-source precedence. A mutation that makes the managed credential override `TS_AUTHKEY` leaves f4's command **green** while that test goes **red**. Same defect class as round-3 Major 3, unswept beyond f1. | Mutation applied in place: `applyConnectProfile` guard loses `\|\| os.Getenv("TS_AUTHKEY") != ""` and `Merge` becomes `if flags.ProfileAuthKey != "" { cfg.AuthKey = flags.ProfileAuthKey }`. `go test ./cmd/cli -run 'ProfileCredential\|BrowserCommand' -count=1` → `ok ts-bridge/cmd/cli 8.754s`; `go test ./cmd/cli -run TestConnectExplicitCredentialSourcesOverrideProfile -count=1` → `--- FAIL ... AuthKey = "tskey-auth-profile-secret", want explicit source`. Restored; `git status --porcelain` empty. `-v` on f4 lists 8 tests and `TestConnectExplicitCredentialSourcesOverrideProfile` is not among them; no other feature command (`f1`–`f3`, `f5`) selects it either. | `TestConnectExplicitCredentialSourcesOverrideProfile` (exists, but selected by no feature command → gate UNTESTED) | contract set (`features.json` f4 `verification`); tests unchanged |
| Minor | REAL | git hygiene (harness override) | The reviewed commit `3213cec` carries `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`, as do `63a2939` and `f0ea65b` — forbidden by "No AI attribution" / "No `Co-Authored-By` trailers referencing AI agents". | `git log -1 --format='%B' 3213cec` → trailer present. Also on `63a2939`, `f0ea65b`. | n/a | git history (amend/rebase before merge) |
| Minor | REAL | implicit credential naming | `auth set --profile <name>` derives the credential name from the profile name, but profile names are only checked for emptiness while credential names must match `[A-Za-z0-9._-]{1,64}` not starting with `.`. A profile named `My Office` (creatable via `init --profile` / `import`) makes the documented onboarding command fail with a credential-name error. | Code read of `cmd/cli/auth.go` (`credentialName := profileName`) against `internal/profile/store.go` (`Set`, empty-only check) and `credential.ValidateName`. | **UNTESTED** | code (or document the constraint); tracked by #399 |
| Minor | THEORETICAL | platform permission proof | On Windows, hardening is proven only by the `icacls` **argv shape** against a stubbed `runICACLS`; `TestStoreUsesOwnerOnlyPermissionsOnUnix` SKIPs there (confirmed in the `-v` f1 run). f1's Windows evidence shows the command was *issued*, not that the effective ACL is owner-only. | `internal/credential/permissions_windows_test.go` stubs the runner and compares argument slices. `-- SKIP: TestStoreUsesOwnerOnlyPermissionsOnUnix`. | `TestHardenCredentialPathUsesOwnerOnlyWindowsACLs` (shape only) | tests (optional: effective ACL) |
| Minor | THEORETICAL | shared-credential overwrite | `auth set --force` silently replaces a credential several profiles reference, rotating the key under every one of them; `auth remove` guards the identical relationship. No warning, no test. | Code read: `runAuthSet` calls `Set(..., force)` unconditionally; `runAuthRemove` refuses unless `--force`. | **UNTESTED** | code + tests; tracked by #391 |
| Minor | THEORETICAL | read-path permission enforcement | Hardening happens only on write. `Store.Get` reads a credential whose mode/ACL was changed externally with no check, so `auth status` can report success on a world-readable secret while `proposal.md`'s risk line says permissions "must fail loudly when they cannot be enforced". | Code read of `Store.Get`; no read-time mode/ACL check in `internal/credential`. | **UNTESTED** | code (defense in depth) |
| Minor | THEORETICAL | browser warning branch | The "no warning when an explicit `--auth-key-file` is supplied" branch of `warnIgnoredBrowserEnvCredential` is not pinned: `TestBrowserAuthKeyFileOverridesProfileCredential` now sets a non-empty `TS_AUTHKEY` but does not capture stderr, so an unconditional warning would still pass. | Code read of `warnIgnoredBrowserEnvCredential` (`authKeyFile != ""` short-circuit); that test asserts only `captured.AuthKey`. | **UNTESTED** | tests |

Notes on the mutation experiments: `internal/credential/store.go` and `cmd/cli/connect.go` + `internal/config/merge.go` were patched in place, the commands above were run, and every file was restored (`cp` from a backup) before this verdict — `git status --porcelain` is empty. No credential value was printed; every key in this review is a synthetic `tskey-auth-*` fixture.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1/AC2/AC4 verified; AC3 is correct in code and now has a named browser test, but its connect explicit-source half is not covered by any feature gate; AC5's `harden` error path is not injected. |
| Verification       | C | Reproducible commands and two genuine controlled-mutation proofs, but f4's declared gate is green under a mutation that breaks the behavior it claims to verify. |
| Scope              | B | Production diff is small and matches the proposal; the spec-range diff also carries merged dependency bumps and #398 from its own issue. |
| Reliability        | B | Atomic temp-then-rename with defer cleanup, loud permission failures, and a deterministic rollback test; set+bind atomicity and the overwrite TOCTOU remain open in #391. |
| Maintainability    | B | `golangci-lint run` on the changed packages → 0 issues, functions short, errors wrapped with `%w`; some package-level mutable test seams. |
| Handoff-readiness  | B | `verification.md` is now accurate, dispositions are recorded, and #391/#399 track the deferred debt. |

### Verdict

**FAIL** — one **REAL Major** in the contract set: `features.json` f4's declared verification command does not select `TestConnectExplicitCredentialSourcesOverrideProfile`, so the harness gate stays green while AC3's connect explicit-source precedence is broken. This is the same defect class round 3 reported for f1, fixed there and never swept across the other feature commands. Because the fix is an edit to `features.json`, a re-review follows — that is the mechanism, not friction.

`dotf spec archive` is **not advisable** in the current state: the verdict blocks, and the contract-set edit invalidates this review's digests by design.

### Recommended next steps

- **Finding 1 — contract set (`features.json` f4):** widen f4's `verification` so it selects the precedence test, e.g. `go test ./cmd/cli -run 'ProfileCredential|BrowserCommand|TestConnectExplicitCredentialSourcesOverrideProfile' -count=1`, or replace the ad-hoc pattern with one that names every behavioral test the behavior claims. Then re-review. Pair it with the sweep below so the next round does not find f5.
- **Sweep (do with finding 1):** for each of f1–f5, confirm the command selects the named tests that pin its stated behavior; f1's depth is the target. Hand-written `-run` regexes are what left f1 and f4 vacuous.
- **Minor (browser warning branch) — tests:** capture stderr in `TestBrowserAuthKeyFileOverridesProfileCredential` and assert the warning is absent, pinning the `--auth-key-file` short-circuit.
- **Minor (git trailer) — declined in `verification.md`; re-surfaced, not gating.** The trailer is still on `3213cec` and the two branch-unique commits; the repo's no-AI-attribution override stands, and the human's explicit session instruction is the recorded disposition.
- **Minors 3–6:** already dispositioned in `verification.md` (#399, #391, effective-ACL and read-time-permission rationale). No further action; they do not gate the archive.
