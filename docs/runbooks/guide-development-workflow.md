---
id: "ts-bridge-dev-workflow"
type: runbook
status: active
tags: [git, workflow, branch-protection]
created: "2026-02-27"
owner: manu
---

# Development Workflow (GitHub Flow)

> How to contribute to ts-bridge using the protected `master` branch workflow.

## Branch Protection Rules (active since 2026-02-27)

| Rule | Setting |
|------|---------|
| Require PR before merge | Yes |
| Required status checks | `test`, `lint`, `security` |
| Branch must be up to date | Yes |
| Required reviewers | 0 (solo project) |
| Allow force push | No |
| Allow deletion | No |
| Enforce for admins | No |

## Standard Workflow

```bash
# 1. Start from up-to-date master
git checkout master
git pull

# 2. Create feature branch
git checkout -b feat/my-feature

# 3. Work, commit (Conventional Commits)
git add -A
git commit -m "feat: add new capability"

# 4. Push and open PR
git push -u origin feat/my-feature
gh pr create --title "feat: add new capability" --body "Description here"

# 5. Wait for CI, then merge via GitHub UI
```

## Branch Naming

| Prefix | Use |
|--------|-----|
| `feat/` | New features |
| `fix/` | Bug fixes |
| `docs/` | Documentation changes |
| `refactor/` | Code restructuring |
| `test/` | Test additions or changes |

## Recovery: Accidentally Worked on `master`

If you make commits on `master` locally and `git push` is rejected:

```bash
# Move your commits to a new branch (zero data loss)
git checkout -b feat/my-feature

# Push the feature branch
git push -u origin feat/my-feature

# Reset local master to match remote
git checkout master
git reset --hard origin/master
```

## Recovery: Feature Branch Behind `master`

If CI requires your branch to be up to date:

```bash
git checkout feat/my-feature
git fetch origin
git rebase origin/master
git push --force-with-lease
```

## How release-please Interacts

1. You merge PRs to `master` with Conventional Commits
2. `release-please` detects version-bumping commits and creates/updates a Release PR
3. Merging the Release PR triggers tag + binary builds + GitHub Release
4. **Never** manually edit `CHANGELOG.md` or version tags — release-please owns those

## Why This Exists

On 2026-02-27, while pushing Headscale compatibility work directly on `master`, `git push` was rejected because release-please had merged two PRs (v1.3.0, v1.3.1) in the meantime. This caused merge conflicts across 4 files requiring manual resolution. Branch protection prevents this class of problem entirely.

See also: [lessons.md](../lessons.md) (2026-02-27 entry)
