package agent

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// DefaultPaths resolves the agent's base from the LocalAppData known folder of
// the process token, never from %LOCALAPPDATA%.
func DefaultPaths() (Paths, error) {
	local, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve the LocalAppData folder of this user: %w", err)
	}
	return Paths{Base: filepath.Join(local, dirName)}, nil
}
