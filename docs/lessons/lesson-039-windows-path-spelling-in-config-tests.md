---
id: "lesson-039-windows-path-spelling-in-config-tests"
type: lesson
status: active
title: "Compare Path Semantics Without Changing Production Configuration"
created: "2026-10-15"
owner: manu
tags: [ts-bridge, lesson, go, windows, testing, profiles]
---

# Compare Path Semantics Without Changing Production Configuration

During #441, the profile-import fixture wrote its state directory with
`filepath.ToSlash`, while the Windows assertion compared it with a native
backslash path. The runtime values were correctly imported, but raw string
equality reported a missing setting.

Inspect each failing field before changing production behavior. When the
contract is a platform path rather than exact YAML spelling, compare
`filepath.Clean` values in the test. This checks syntactic path equivalence,
not symlink resolution or case-insensitive filesystem identity.

Keep the supplied path spelling in production; do not normalize persisted
configuration just to satisfy a representation-specific test.

Evidence: `TestProfileRuntimeImportPreservesExistingTargetAndCredential` in
`cmd/cli/profile_runtime_test.go`; see the CFG-005 verification record.
