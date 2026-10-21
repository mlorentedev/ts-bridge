//go:build windows

package host

import "os/exec"

func isElevatedImpl() bool {
	return exec.Command("net", "session").Run() == nil
}
