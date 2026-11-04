//go:build !linux && !windows

package bootstrapssh

import (
	"io"
	"os/exec"
)

// This system has no parent-death signal or job object: an abrupt exit can
// leave ssh running, and only the graceful shutdown stops it (#376).
func prepareContainment(*exec.Cmd) {}

func containProcess(*exec.Cmd) (io.Closer, error) { return nopCloser{}, nil }
