---
id: "ts-bridge-release-missing-assets"
type: troubleshooting
status: active
tags: [release, ci, github-actions, release-please]
created: "2026-02-23"
owner: manu
---

# Release Appears Without Assets After Merge

## Symptom

After merging the release-please PR, the GitHub release is published but shows no binary assets (no linux, windows, darwin archives or checksums).

## Cause

This is **expected behavior**, not a bug. The release workflow has two sequential jobs:

1. `release-please` — Creates the tag and GitHub release immediately (~15s)
2. `build-release` — Compiles 6 platform binaries, packages archives, uploads assets (~4-5 min)

During the gap between jobs, the release is visible but empty.

## Fix

No fix needed. Wait ~5 minutes and refresh the release page. All assets will appear once `build-release` completes:

- `ts-bridge-vX.Y.Z-linux-amd64.tar.gz`
- `ts-bridge-vX.Y.Z-linux-arm64.tar.gz`
- `ts-bridge-vX.Y.Z-darwin-amd64.tar.gz`
- `ts-bridge-vX.Y.Z-darwin-arm64.tar.gz`
- `ts-bridge-vX.Y.Z-windows-amd64.zip`
- `ts-bridge-vX.Y.Z-windows-arm64.zip`
- `checksums.txt`

## Verification

```bash
# Check workflow status
gh run list --workflow=release.yml --limit=1

# Check assets after completion
gh release view vX.Y.Z --json assets --jq '.assets[].name'
```

## Notes

- Multiple commits in the PR before the release do not affect asset generation. Release-please aggregates all conventional commits into the changelog correctly.
- If assets are still missing after the workflow completes successfully, check the `build-release` job logs for compilation or upload errors.
