package bootstrapssh

import "golang.org/x/sys/windows"

// processGone reports a pid that has exited or that no longer exists.
func processGone(pid int) bool {
	//nolint:gosec // a pid from this test's own child fits a DWORD.
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true
	}
	defer func() { _ = windows.CloseHandle(h) }()
	ev, err := windows.WaitForSingleObject(h, 0)
	return err == nil && ev == windows.WAIT_OBJECT_0
}
