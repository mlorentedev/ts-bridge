//go:build darwin

package host

import (
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
)

func isElevatedImpl() bool {
	return syscall.Geteuid() == 0
}

func setupImpl(_ Config, _ *slog.Logger) (SetupResult, error) {
	return SetupResult{RDPPort: 3389}, nil
}

func checkImpl(_ Config, _ *slog.Logger) (CheckResult, error) {
	result := CheckResult{}
	tsIP := TailscaleIP()
	result.TailscaleIP = tsIP
	result.TailscaleUp = tsIP != ""
	result.RDPPort = defaultRDPPort
	return result, nil
}

func tailscaleIPImpl() string {
	out, err := exec.Command("tailscale", "ip", "-4").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
