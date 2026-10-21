//go:build darwin

package host

import "syscall"

func isElevatedImpl() bool {
	return syscall.Geteuid() == 0
}
