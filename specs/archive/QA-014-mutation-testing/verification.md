---
id: "qa-014-mutation-testing"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-06-19"
tags: [spec, testing, qa, mutation, gremlins, ci]
issue: 189
---

# QA-014: Verification

## Evidence

Verified 2026-08-06 against the seven scheduled runs of
`.github/workflows/mutation.yml` (2026-06-22 → 2026-08-03, **7/7 success**,
zero `workflow_dispatch` runs) and the `mutation-report` artifact of the latest
run (`30789587228`, 2026-08-03).

- [x] Workflow parses: `.github/workflows/mutation.yml` validated by
      actionlint / GitHub Actions on push.
      *Seven scheduled runs executed it; a workflow that failed to parse would
      never have produced a run.*
- [x] `go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0` succeeds
      in CI and `git diff --exit-code go.mod go.sum` reports no change.
      *Steps `Install gremlins` and `Verify go.mod is untouched`
      (`mutation.yml:38` and `:41`); the latter fails the job on any diff, and
      all seven runs are green.*
- [~] `gremlins unleash --dry-run` lists mutants (via `workflow_dispatch`).
      **Never executed** — all seven runs are `schedule`, none `workflow_dispatch`.
      Superseded rather than satisfied: the full `gremlins unleash` runs weekly
      and enumerates the same mutant set plus its results, which is strictly
      stronger than a dry-run listing. The `workflow_dispatch` trigger remains
      wired for on-demand use.
- [x] `gremlins unleash` produces `mutation-report.json`, uploaded as artifact.
      *`mutation.yml:49` and `:68`; artifact `mutation-report` (8670 bytes)
      downloaded and parsed from run `30789587228`.*
- [x] Job is green even with survived mutants (advisory thresholds = 0).
      *That run reports 121 `LIVED` mutants and concludes `success`.*
- [x] `*_linux.go` / `*_windows.go` / `*_darwin.go` absent from mutated files.
      *Of the 30 files in the report, zero match
      `_(linux|windows|darwin)\.go$` — the `.gremlins.yaml` excludes hold.*

## Test Output

Summary from run `30789587228` (2026-08-03, `mutation-report.json`):

```
go_module           ts-bridge
mutants_total       478
mutants_killed      357
mutants_lived       121
mutants_not_covered 241
mutants_not_viable  0
test_efficacy       74.69 %
mutations_coverage  66.48 %
elapsed_time        111.0 s
files mutated       30
```

Survivors by file (the input to the follow-up hardening work in `tasks.md`):

| File | KILLED | LIVED | NOT COVERED |
|---|---|---|---|
| `internal/config/merge.go` | 50 | 43 | 8 |
| `internal/config/config.go` | 79 | 31 | 9 |
| `internal/logging/logging.go` | 16 | 18 | 7 |
| `internal/proxy/reconnect.go` | 16 | 12 | 0 |
| `internal/proxy/proxy.go` | 7 | 6 | 5 |
| `internal/discover/env.go` | 5 | 5 | 1 |
| `cmd/cli/*` (aggregate) | 94 | 0 | 205 |

All 43 survivors in `merge.go` are conditionals: 25 `CONDITIONALS_NEGATION` and
18 `CONDITIONALS_BOUNDARY` — i.e. the config-precedence logic that QA-011 (#181)
targets.

## Commit Hashes

- Harness setup (`.gremlins.yaml` + `Makefile` + workflow + spec):
  `c18cac1` — *test: advisory mutation testing harness (gremlins) (#190)*
- First green scheduled run: `27929776050` (2026-06-22, head `11b1bf7`)
- Report used for this verification: `30789587228` (2026-08-03, head `d7fbc22`)
