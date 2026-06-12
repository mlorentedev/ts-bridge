---
tags: [spec, verification, CLI-003]
created: "2026-06-10"
---

# Verification - CLI-003

## Evidence

- [x] Interactive mode prompts for auth key (masked), target, instance, format
- [x] Non-interactive mode: ts-bridge init --auth-key X --target Y writes config silently
- [x] YAML output valid and includes comments
- [x] .env output matches .env.example format
- [x] go test ./... PASS
- [x] golangci-lint run clean
- [x] Auth key never written to YAML (goes to .env instead)
- [x] Permission warning on world-readable files (Unix)
- [x] Validation: invalid auth key, invalid target, invalid format all rejected

## Archive checklist

- [x] proposal.md frontmatter set to status: archived
- [x] Folder moved: specs/CLI-003/ -> specs/archive/CLI-003/
- [x] Issue #50 closed with PR link #66