//go:build !windows

package oslock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// Try locks file exclusively without waiting. It returns ErrHeld when another
// holder has it. The lock lasts until file is closed or the process exits.
func Try(file *os.File) error {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return ErrHeld
	}
	return err
}
