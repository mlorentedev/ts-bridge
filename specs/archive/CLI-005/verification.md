---
tags: [spec, verification, CLI-005]
created: "2026-06-10"
---

# Verification - CLI-005

## Evidence

- [ ] --config ts-bridge.yaml loads and applies settings
- [ ] CLI flag overrides YAML value
- [ ] Env var overrides YAML value
- [ ] Missing YAML file is not an error
- [ ] Unknown YAML fields produce warning
- [ ] Auth key in YAML rejected with clear error
- [ ] go test ./... PASS
- [ ] PR diff < 250 lines (excluding tests)

## Archive checklist

- [ ] proposal.md frontmatter set to status: archived
- [ ] Folder moved: specs/CLI-005/ -> specs/archive/CLI-005/
- [ ] Issue #52 closed with PR link