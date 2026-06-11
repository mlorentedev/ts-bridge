---
tags: [spec, tasks, OPS-003]
created: "2026-06-10"
---

# Tasks - OPS-003

## Setup

- [ ] Branch from master: chore/go-mod-tidy-ci

## Implementation

- [ ] Add step to .github/workflows/ci.yml:
      run: go mod tidy && git diff --exit-code go.mod go.sum
- [ ] Verify existing go.mod/go.sum pass the check

## Testing

- [ ] CI passes on the PR branch

## Closing

- [ ] PR < 10 lines diff
- [ ] PR references issue #56