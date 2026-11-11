package bootstrapssh

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

// processGone reports a pid that has exited or that no longer exists. Any
// other OpenProcess error, such as access denied, fails the test: it says
// nothing about the child, so it must not read as a kill.
func processGone(t testing.TB, pid int) bool {
	t.Helper()
	//nolint:gosec // a pid from this test's own child fits a DWORD.
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return true // no process has this pid
	}
	if err != nil {
		t.Fatalf("open pid %d: %v", pid, err)
	}
	defer func() { _ = windows.CloseHandle(h) }()
	ev, err := windows.WaitForSingleObject(h, 0)
	if err != nil {
		t.Fatalf("wait on pid %d: %v", pid, err)
	}
	return ev == windows.WAIT_OBJECT_0
}
