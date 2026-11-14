---
id: "LOG-788-rotator-shared-file"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-11-14"
issue: "mlorentedev/ts-bridge#788"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# LOG-788-rotator-shared-file

> **Naming**: file lives at `<repo>/specs/LOG-788-rotator-shared-file/proposal.md`. `LOG-788-rotator-shared-file` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

<!-- from issue #788: BUG: one ts-bridge process exiting deletes the shared daily log other sessions still write -->

Every ts-bridge process appends to one daily file, and `FileRotator.Close` gzips that day and removes it while other sessions still write to it. On Unix their lines go to an unlinked inode and are lost; on Windows a partial `.gz` is written and later overwritten. ADR-016 already allows two sessions at once, and ADR-018 makes every session stop a child exit, so this becomes the normal path. The v2.0.0 audit (#78) found two more defects with the same root cause: a same-day restart truncates the earlier archive, and a process that crosses midnight never compresses yesterday or enforces retention.

## What

- `Close` only closes the file. No process compresses or removes the day it is writing.
- Compression runs for **past days only**, from one rotation path used by both `Open` and the midnight rollover in `Write`, followed by retention.
- Each compression claims the day's `.log` with an atomic rename to a unique name, so two processes never compress the same file at once. It writes a gzip into a private temp file and publishes it **without clobbering**: as `ts-bridge-<day>.log.gz` when no archive of that day exists (`os.Link` refuses an existing name), otherwise under a unique `ts-bridge-<day>.log.<pid>.<ns>.gz`. No archive is ever appended to, truncated or overwritten.
- Retention removes archives (`.gz`) only. A `.log` or a claim still holds lines no archive has, so it is never aged out.

## Out of scope

- The per-profile log file name ADR-018 specifies ("each session writes its own log file, named after the profile"). The profile does not reach `LoggerInit` today; that wiring belongs to the #782 session supervisor work. This spec makes the shared file safe, which per-profile names will not remove (non-profile runs still share one).
- Changing the retention limits (7 days, 10 archives).

## Risks / open questions

- A line written in the instant a peer claims yesterday's file at midnight cannot be lost: every `Write` rotates to today before it writes, so a peer never appends to a claimed file.
- **Windows:** Go opens files with `FILE_SHARE_DELETE`, so the claim rename succeeds while a peer still holds the file. This is safe for the same reason as above.
- **Failed or interrupted compression:** the lines stay in the claim. A rotation retries a claim only once it is older than `staleClaimAge` (1h; a claim is touched when it is made, and a compression takes milliseconds), so a peer's in-flight claim is never taken. A failed archive write removes its temp file, and a half-written archive is never published.
- **Accepted:** a claim retried while its maker is in fact still alive after an hour (a process hung mid-compression) produces a second, identical archive. That is duplication, never corruption or loss.
- **Accepted:** `maxFiles = 10` counts archives, not days, so a day with several runs keeps fewer days of history.
- **Accepted:** a `.log` or claim that can never be archived (a disk that stays full) accumulates until the cause clears. Those files are the unarchived lines themselves.

## Acceptance criteria

- [x] AC1: closing one rotator leaves the file another rotator is writing in place; the other's next write lands in a file that still exists and holds both rotators' lines.
- [x] AC2: an existing archive is never truncated or overwritten; a later run of the same day gets its own archive.
- [x] AC3: the midnight rollover in `Write` compresses the previous day and runs retention, through the same path `Open` uses.
- [x] AC4: today's file is never compressed by rotation, even when its name sorts among the past days.
- [x] AC5: a compression that fails keeps the day's lines, even past retention, and a later rotation archives them once the claim is stale; a fresh claim is never taken; two concurrent compressions of one day never corrupt or lose it; a directory named like a log is left alone.

## References

- Bitácora board: #788
- Related ADR: `docs/adr/adr-018-local-control-plane.md` (Logs, B8)
