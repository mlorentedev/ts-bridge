//go:build !windows

package cmd

import "io"

// supportsClearScreen returns true on non-Windows platforms.
// ANSI escape codes work reliably on Unix terminals.
func supportsClearScreen(io.Writer) bool {
	return true
}
