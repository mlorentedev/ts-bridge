package bootstrapssh

import (
	"io"
	"os/exec"
	"syscall"
)

// prepareContainment asks the kernel to SIGKILL ssh when its parent dies.
// Pdeathsig follows the thread that forked, not the process; Go retires a
// thread only when a LockOSThread goroutine exits, which the bootstrap never
// does, so in practice it fires when ts-bridge dies.
func prepareContainment(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}

// containProcess has nothing to do after the start on Linux.
func containProcess(*exec.Cmd) (io.Closer, error) { return nopCloser{}, nil }
