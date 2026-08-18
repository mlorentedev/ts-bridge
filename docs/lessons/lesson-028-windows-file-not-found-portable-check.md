---
id: lesson-028-windows-file-not-found-portable-check
type: lesson
status: active
created: "2026-08-13"
owner: manu
tags: [ts-bridge, lesson]
---

# Windows file not found portable check

**Context:** Writing tests to assert that a missing configuration file throws an error.
**Problem:** On Windows, os.Open returns an error containing 'The system cannot find the file specified', unlike Unix which returns 'no such file or directory'. A hardcoded string check fails on Windows tests.
**Solution:** Use errors.Is(err, os.ErrNotExist) which is portable across operating systems for checking file-not-found errors.
