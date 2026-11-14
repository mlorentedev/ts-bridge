package agent

import "golang.org/x/sys/unix"

// peerUID returns the uid of the process that connected to the socket fd.
func peerUID(fd int) (uint32, error) {
	cred, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return 0, err
	}
	return cred.Uid, nil
}
