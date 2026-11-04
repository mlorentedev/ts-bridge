package bootstrapssh

import (
	"fmt"
	"io"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// prepareContainment has nothing to set before the start on Windows.
func prepareContainment(*exec.Cmd) {}

// containProcess assigns the started ssh to a Job Object that kills its
// members when the last handle closes. The handle is this process's, so the
// kernel closes it when ts-bridge dies however it dies. Two boundaries stay:
// a process ssh spawns before the assignment (a ProxyCommand) is not in the
// job, and a parent job that forbids nesting makes the assignment fail.
func containProcess(cmd *exec.Cmd) (io.Closer, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	// #nosec G103 -- SetInformationJobObject takes the struct by address and size.
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("set kill-on-close: %w", err)
	}
	if err := assignToJob(job, cmd.Process.Pid); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	return jobHandle(job), nil
}

func assignToJob(job windows.Handle, pid int) error {
	// #nosec G115 -- a Windows pid is a DWORD.
	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("open ssh process: %w", err)
	}
	defer func() { _ = windows.CloseHandle(proc) }()
	if err := windows.AssignProcessToJobObject(job, proc); err != nil {
		return fmt.Errorf("assign ssh to job object: %w", err)
	}
	return nil
}

// jobHandle closes the job, killing whatever is still in it.
type jobHandle windows.Handle

func (j jobHandle) Close() error { return windows.CloseHandle(windows.Handle(j)) }
