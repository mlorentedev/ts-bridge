package agent

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// pipeNamePrefix names the agent's pipe; the user's SID completes it, so two
// users on one machine never contend for a name (ADR-018 §2).
const pipeNamePrefix = "ts-bridge-agent-"

// DefaultPaths resolves the agent's base from the LocalAppData known folder of
// the process token, never from %LOCALAPPDATA%, and names the pipe after the
// token's SID.
func DefaultPaths() (Paths, error) {
	local, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve the LocalAppData folder of this user: %w", err)
	}
	self, err := currentIdentity()
	if err != nil {
		return Paths{}, err
	}
	return Paths{Base: filepath.Join(local, dirName), Run: pipeNamePrefix + string(self)}, nil
}
