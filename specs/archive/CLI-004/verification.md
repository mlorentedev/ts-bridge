---
tags: [spec, verification, CLI-004]
created: "2026-06-10"
---

# Verification - CLI-004

## Evidence

- [ ] ts-bridge host setup on Windows enables RDP, firewall, Tailscale unattended
- [ ] ts-bridge host setup errors without admin
- [ ] ts-bridge host check prints Tailscale IP, RDP port, firewall status
- [ ] ts-bridge host check --json outputs valid JSON
- [ ] Linux xrdp detection works
- [ ] go test ./... PASS
- [ ] PR diff < 250 lines (excluding tests)

## Archive checklist

- [ ] proposal.md frontmatter set to status: archived
- [ ] Folder moved: specs/CLI-004/ -> specs/archive/CLI-004/
- [ ] Issue #51 closed with PR link