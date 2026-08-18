---
id: lesson-029-os-chdir-error-check-in-tests
type: lesson
status: active
created: "2026-08-13"
owner: manu
tags: [ts-bridge, lesson]
---

# os.Chdir error check in tests

**Context:** Adding os.Chdir in a t.Cleanup function to restore the working directory.
**Problem:** golangci-lint flags unhandled errors for os.Chdir calls even inside t.Cleanup blocks, breaking CI.
**Solution:** Always assign and check the error returned by os.Chdir, e.g. if err := os.Chdir(dir); err != nil { t.Fatalf(...) }
