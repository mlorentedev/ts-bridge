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
- Compression runs for **past days only**, from one rotation path used by both `Open` and the midnight rollover in `Write`, followed by retention cleanup.
- A past day's `.log` is claimed by an atomic rename before it is compressed, so two processes rotating at once never compress the same file twice. The archive is opened for append and gets a new gzip member, so an existing `.gz` for that day is extended, never truncated (multi-member gzip reads as one stream with `zcat` and Go's `gzip.Reader`).

## Out of scope

- The per-profile log file name ADR-018 specifies ("each session writes its own log file, named after the profile"). The profile does not reach `LoggerInit` today; that wiring belongs to the #782 session supervisor work. This spec makes the shared file safe, which per-profile names will not remove (non-profile runs still share one).
- Changing the retention limits (7 days, 10 archives).

## Risks / open questions

- A line written in the instant a peer claims yesterday's file at midnight cannot be lost: every `Write` rotates to today before it writes, so a peer never appends to a claimed file.
- **Windows:** Go opens files with `FILE_SHARE_DELETE`, so the claim rename succeeds while a peer still holds the file. This is safe for the same reason as above. (Round 1 of the review corrected an earlier claim that the rename fails on Windows.)
- **Failed compression** (archive unwritable, disk full): the lines stay in the `.claimed` file. Retention never deletes a claim, and every rotation retries leftover claims for past days, so nothing is lost. A write that fails midway truncates the archive back to its old size, so the archive never holds a partial member.
- **Accepted:** a rotation that retries another live process's claim while that process is still compressing it appends the day twice. The result is duplicated lines, never lost ones, and it needs two processes rotating within the same milliseconds.
- **Accepted:** a claim that keeps failing (for example a disk that stays full) accumulates until the cause clears. Those files are the unarchived lines themselves.

## Acceptance criteria

- [x] AC1: closing one rotator leaves the file another rotator is writing in place; the other's next write lands in a file that still exists and holds both rotators' lines.
- [x] AC2: compressing a past day whose `.gz` already exists appends to it; the archive decompresses to the earlier content followed by the new content.
- [x] AC3: the midnight rollover in `Write` compresses the previous day and runs retention, through the same path `Open` uses.
- [x] AC4: today's file is never compressed by rotation, even when its name sorts among the past days.
- [x] AC5: a compression that fails keeps the day's lines, even past retention, and a later rotation archives them; two concurrent compressions of one day archive it once; a directory named like a log is left alone.

## References

- Bitácora board: #788
- Related ADR: `docs/adr/adr-018-local-control-plane.md` (Logs, B8)
