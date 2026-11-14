package agent

import "golang.org/x/sys/unix"

// sunPathMax is the size of sun_path, terminating NUL included.
const sunPathMax = 104

// peerUID returns the uid of the process that connected to the socket fd.
func peerUID(fd int) (uint32, error) {
	cred, err := unix.GetsockoptXucred(fd, unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	if err != nil {
		return 0, err
	}
	return cred.Uid, nil
}
