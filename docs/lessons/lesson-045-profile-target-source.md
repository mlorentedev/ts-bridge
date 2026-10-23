---
id: "lesson-045-profile-target-source"
type: lesson
status: active
title: "Preserve the original profile target at the resolution boundary"
created: "2026-10-22"
owner: manu
tags: [ts-bridge, lesson, profiles, configuration]
---

# Preserve the Original Profile Target at the Resolution Boundary

`connect` fills empty YAML fields from the selected profile before merging
environment and flags. Once those layers are flattened, the effective target
alone cannot establish whether the selected profile was overridden (#417).
Return the original profile target from the existing load rather than reading
the mutable store again solely for a warning.

Report the winning source only after validation and ownership succeed, using the
initialized structured logger before startup. Equal effective targets and a
SOCKS5 override are not changed TCP destinations. `.env` values enter the
environment layer, so `TS_TARGET` is accurate attribution without pretending to
distinguish the file from the process environment.
