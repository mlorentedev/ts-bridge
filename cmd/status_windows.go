//go:build windows

package cmd

import (
	"io"
	"os"
	"syscall"
)

// supportsClearScreen checks if the Windows console supports VT processing.
// Without VT enabled, ANSI escape codes produce garbage output.
// Returns true only if the console is a TTY with VT processing enabled.
//nolint:unused // used on Windows only (build tag: windows); golangci-lint on Linux sees it as dead code
func supportsClearScreen(w io.Writer) bool {
	// Only check os.Stdout (the common case for Cobra commands).
	// Other writers (bytes.Buffer in tests, io.Discard, etc.) are safe.
	if w != os.Stdout {
		return true
	}

	handle := syscall.Stdout
	var mode uint32
	if err := syscall.GetConsoleMode(handle, &mode); err != nil {
		// Not a console (pipe, file, etc.) — skip clear.
		return false
	}
	// ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
	return (mode & 0x0004) != 0
}
