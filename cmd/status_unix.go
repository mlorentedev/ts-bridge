//go:build !windows

package cmd

import (
	"io"
)

// supportsClearScreen returns true on non-Windows platforms.
// ANSI codes work on any Unix terminal; we conservatively return true.
func supportsClearScreen(w io.Writer) bool {
	// On Unix, ANSI codes work on any writer (TTY or not).
	// We conservatively return true — if it's a pipe/file the
	// escape codes are harmless (just ignored by the consumer).
	return true
}
