---
id: "qa-014-mutation-testing"
type: spec
status: proposed
created: "2026-06-19"
tags: [spec, testing, qa, mutation, gremlins, ci]
issue: 189
---

# QA-014: Verification

## Evidence

- [ ] Workflow parses: `.github/workflows/mutation.yml` validated by
      actionlint / GitHub Actions on push.
- [ ] `go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0` succeeds
      in CI and `git diff --exit-code go.mod go.sum` reports no change.
- [ ] `gremlins unleash --dry-run` lists mutants (via `workflow_dispatch`).
- [ ] `gremlins unleash` produces `mutation-report.json`, uploaded as artifact.
- [ ] Job is green even with survived mutants (advisory thresholds = 0).
- [ ] `*_linux.go` / `*_windows.go` / `*_darwin.go` absent from mutated files.

## Test Output

```
# Paste `gremlins unleash --dry-run` mutant list here (first dispatch run)
# Paste the mutation score summary line here
```

## Commit Hashes

- Harness setup (.gremlins.yaml + Makefile + workflow + spec):
- First green scheduled/dispatched run:
