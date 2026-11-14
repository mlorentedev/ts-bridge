package oslock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// Try locks file exclusively without waiting. It returns ErrHeld when another
// holder has it. The lock lasts until file is closed or the process exits.
func Try(file *os.File) error {
	err := windows.LockFileEx(windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrHeld
	}
	return err
}
