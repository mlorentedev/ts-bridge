---
tags: [spec, tasks, OPS-002]
created: "2026-06-10"
---

# Tasks - OPS-002

## Setup

- [x] Branch from master: chore/remove-obsolete-scripts
- [x] Depends on CLI-002, CLI-003, CLI-004 merged

## Implementation

- [x] Remove scripts/client/run.sh
- [x] Remove scripts/client/run.ps1
- [x] Remove scripts/client/bootstrap.sh
- [x] Remove scripts/client/bootstrap.ps1
- [x] Archive or remove scripts/tests/ BATS tests for removed scripts
- [x] Update README.md to reference CLI commands
- [x] Update CONTRIBUTING.md
- [x] Update AGENTS.md Key Paths section
- [x] Update .env.example to note CLI is recommended path

## Testing

- [x] go test ./... green
- [x] No broken links in docs

## Closing

- [x] PR #73 merged (14 insertions, 576 deletions)
- [x] PR references issue #55