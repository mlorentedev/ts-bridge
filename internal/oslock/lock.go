// Package oslock holds the advisory file lock behind ts-bridge's single-owner
// guarantees: per-profile ownership (ADR-016), the profile store lock and the
// per-user agent (ADR-018). The kernel drops the lock when its holder exits, so
// a crashed process leaves nothing to clear.
package oslock

import "errors"

// ErrHeld reports that another open file description holds the lock.
var ErrHeld = errors.New("lock held by another process")
