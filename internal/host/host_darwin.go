//go:build darwin

package host

import (
	"os/exec"
	"strings"
	"syscall"
)

func isElevatedImpl() bool {
	return syscall.Geteuid() == 0
}

func setupImpl(_ SetupFlags) (SetupResult, error) {
	return SetupResult{RDPPort: 3389}, nil
}

func checkImpl() (CheckResult, error) {
	result := CheckResult{}
	tsIP := TailscaleIP()
	result.TailscaleIP = tsIP
	result.TailscaleUp = tsIP != ""
	result.RDPPort = 3389
	return result, nil
}

func tailscaleIPImpl() string {
	out, err := exec.Command("tailscale", "ip", "-4").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
