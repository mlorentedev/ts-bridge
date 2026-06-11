---
tags: [spec, tasks, OPS-002]
created: "2026-06-10"
---

# Tasks - OPS-002

## Setup

- [ ] Branch from master: chore/remove-obsolete-scripts
- [ ] Depends on CLI-002, CLI-003, CLI-004 merged

## Implementation

- [ ] Remove scripts/client/run.sh
- [ ] Remove scripts/client/run.ps1
- [ ] Remove scripts/client/bootstrap.sh
- [ ] Remove scripts/client/bootstrap.ps1
- [ ] Archive or remove scripts/tests/ BATS tests for removed scripts
- [ ] Update README.md to reference CLI commands
- [ ] Update CONTRIBUTING.md
- [ ] Update AGENTS.md Key Paths section
- [ ] Update .env.example to note CLI is recommended path

## Testing

- [ ] go test ./... green
- [ ] No broken links in docs

## Closing

- [ ] PR < 100 lines diff
- [ ] PR references issue #55