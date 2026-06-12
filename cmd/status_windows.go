//go:build windows

package cmd

import (
	"io"
	"os"
	"runtime"
	"syscall"
)

// supportsClearScreen returns true if the writer supports ANSI escape
// sequences for screen clearing. On Windows without VT processing,
// ANSI codes produce garbage output — in that case we return false.
//nolint:unused // used on Windows only (build tag: windows)
func supportsClearScreen(w io.Writer) bool {
	// Only check os.Stdout (the common case for Cobra commands).
	// Other writers (bytes.Buffer in tests, io.Discard, etc.) are safe.
	if w != os.Stdout {
		return true
	}

	if runtime.GOOS == "windows" {
		// On Windows, check if the console supports VT processing.
		handle := syscall.Stdout
		var mode uint32
		if err := syscall.GetConsoleMode(handle, &mode); err != nil {
			// Not a console (pipe, file, etc.) — skip clear.
			return false
		}
		// ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
		return (mode & 0x0004) != 0
	}

	// On Unix, ANSI codes work on any writer (TTY or not).
	// We conservatively return true — if it's a pipe/file the
	// escape codes are harmless (just ignored by the consumer).
	return true
}
