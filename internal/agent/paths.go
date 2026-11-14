package agent

import (
	"fmt"
	"path/filepath"
	"strconv"
)

const (
	dirName    = "ts-bridge"
	lockName   = "agent.lock"
	socketName = "agent.sock"
)

// Paths names where the agent keeps its state: Base holds the agent lock, and
// Run holds the socket on Unix (Windows uses a named pipe and leaves it empty).
type Paths struct {
	Base string
	Run  string
}

// pathsFor applies ADR-018's layout to a user's uid and home directory. It
// takes the OS and the /run/user probe as arguments so every platform's choice
// is tested on any host. Nothing here reads the environment: a desktop session
// and a login supervisor see different ones, and a layout that followed
// $XDG_* would give them two locks and two agents.
func pathsFor(goos string, uid int, home string, runUserUsable func(string) bool) Paths {
	if goos == "darwin" {
		base := filepath.Join(home, "Library", "Application Support", dirName)
		return Paths{Base: base, Run: filepath.Join(base, "run")}
	}
	base := filepath.Join(home, ".local", "state", dirName)
	runUser := filepath.Join("/run/user", strconv.Itoa(uid))
	if runUserUsable(runUser) {
		return Paths{Base: base, Run: filepath.Join(runUser, dirName)}
	}
	return Paths{Base: base, Run: filepath.Join("/tmp", fmt.Sprintf("%s-%d", dirName, uid))}
}
