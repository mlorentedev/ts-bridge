---
id: "SEC-391-store-lock"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-11-06"
issue: "mlorentedev/ts-bridge#391"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# SEC-391-store-lock

## Why

`auth set`, `auth remove`, `import` and `init --profile` change two stores that nothing coordinates:
the profile YAML and the credential directory. Each command reads, decides and writes with no lock,
so two commands can both pass a check and both write. Three failures follow (#391):

- Concurrent writers lose updates. Reproduced before any change: 24 goroutines each calling
  `Store.Set` for a distinct profile left 2 to 4 profiles in the file, and one run left YAML with a
  duplicate mapping key that no later command could parse.
- `auth set` stores the key, then associates it. If the profile moved in between, the association
  failed or bound the key to the wrong control plane, and with `--force` the previous key was already
  gone (#653 scenario 2; the review of #664: checking again before `Set` only narrows the window).
- `auth remove` computed the references it acts on before writing, so a reference added meanwhile
  was cleared or its key deleted from under it. It cleared references before deleting the key, so a
  failed deletion left a secret the user believed removed.

## What

One per-user store lock, `locks/store.lock` beside `profiles.yaml`, taken by every mutation. It is the
same advisory-lock helper `connect` uses for profile ownership (`flock` on Unix, `LockFileEx` on
Windows), made to wait: `lockStore` retries the non-blocking lock every 25 ms for up to 10 s, then
fails with "another ts-bridge command is updating the profile store; retry in a moment". The kernel
drops the lock when its holder exits, so a crashed command leaves nothing to clear.

- **Writers.** `Import`, `Set`, `Configure`, `SetCredential` and `ClearCredential` each run inside
  `Store.Update`. `Update` hands the function a `Tx` whose methods are the only way to mutate while
  the lock is held, because the lock does not nest.
- **Atomic save.** The store file is written to a temporary file and renamed over the old one
  (`secretfile.WriteFileWith`), so a reader or a crash sees the old store or the new one, never a
  truncated file. The rename retries five times: Windows refuses to replace a file a reader has open.
- **Readers never lock.** `Get`, `List`, `ListProfiles`, `auth status` and `auth list` stay available
  while a mutation holds the lock, and see the last complete store.
- **`BindCredential`** (`auth set`) is one transition under the lock: re-read the profile, refuse if
  its control URL differs from the one the command read before prompting for the key, refuse a
  Headscale key on a SaaS profile, then store the key and write the reference. Every refusal comes
  before the stored key is touched. If the reference write fails, the previous key is restored, or
  the new one removed. The key is read before the lock is taken, so a person typing never blocks other commands.
- **`UnbindCredential`** (`auth remove`) resolves the credential, counts its references and acts, all
  under the lock. The key is deleted before any reference is cleared: a failed deletion changes
  nothing, and an interruption leaves only a dangling reference, which running the removal again
  clears.
- **Exclusive creation.** Without `--force`, `credential.Store.Set` checks the name is absent. Every
  writer holds the store lock, so two creators cannot both pass the check; a test pins exactly one
  winner.

## Decisions

- **Lock primitive.** The brief assumed `golang.org/x/sys` was indirect and asked for the standard
  library only. It is a direct requirement in `go.mod`, and `internal/profile` already uses
  `unix.Flock` and `windows.LockFileEx` for the ownership lock. Reusing that helper adds no
  dependency and gives Windows a real lock, where a lock file with stale detection would have to
  guess whether its holder is alive. `lockFile` now returns a neutral `errLockHeld`; `Lock` maps it
  to its "already connected" message, `lockStore` to the busy message.
- **No `O_EXCL` or `os.Link` for exclusive creation.** Hard links are not available on every
  filesystem a Windows home directory can sit on, and the lock already serializes every
  ts-bridge writer.
- **No intent journal in this PR.** Each interruption point leaves a state a rerun repairs: a crash
  after the key is stored leaves an unreferenced key, which `auth list` shows with no profile and
  `auth remove <name>` deletes; a crash during removal leaves a dangling reference that a rerun
  clears; a crash during a save leaves a stray temporary file that nothing reads.
- **ADR-007 package table.** `profile` now imports `secretfile`; the row is amended, as the layout
  test requires.

## Out of scope

- `init --profile` without `--force`: its "profile already exists" check still reads before the
  lock, so two concurrent `init` runs can both pass it (PR 2, see tasks).
- A scan that reports orphan keys and dangling references (PR 2).
- PR #664 (`CheckCredentialShare`, `AssociateCredential`). It is open and edits the same lines of
  `runAuthSet`. Whichever merges second moves its check and association inside
  `BindCredential`'s transaction, where the profile is already re-read under the lock.

## Risks / open questions

- A holder that stalls (a debugger, a suspended process) makes other mutations fail after 10 s with
  the busy message rather than wait forever. Reads are unaffected.
- The advisory lock does not stop a process that ignores it, such as an editor saving `profiles.yaml`.
  The atomic replace keeps the file whole; the editor's change may be overwritten.
- `flock` is unreliable on some network filesystems. The store lives in the per-user config directory.

## Acceptance criteria

- [ ] AC1: credential and profile mutations, including `import`, `init --profile` and runtime
  configuration, serialize across processes through a per-user store lock; concurrent writers lose
  no update.
- [ ] AC2: non-forced credential creation is exclusive: of several concurrent creators of one name,
  exactly one succeeds.
- [ ] AC3: a set or remove transition checks before it writes, undoes the key if the reference write
  fails, deletes the key before clearing references, and a rerun repairs an interrupted removal.
- [ ] AC4: tests cover concurrent setters, a reference change during removal, a `control_url` change
  between the read and the key being stored, association failure, deletion failure, and crash
  artifacts (a killed lock holder, an abandoned temporary file, a dangling reference).
- [ ] AC5: read-only commands work, or fail clearly, while a mutation holds the lock; a mutation that
  cannot get the lock says so and how to proceed.

## References

- Issue #391 and its comments (the CFG-005 follow-up from #442; #653 scenario 2; the review of #664)
- ADR-015 (managed credentials), ADR-007 (package table)
