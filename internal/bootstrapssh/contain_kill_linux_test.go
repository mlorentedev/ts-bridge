package bootstrapssh

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// processGone reports a pid that has exited: absent, or a zombie nobody reaped.
func processGone(pid int) bool {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	// The state follows the parenthesised command name.
	fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	return len(fields) > 0 && fields[0] == "Z"
}

func TestPrepareContainment_SetsPdeathsig(t *testing.T) {
	cmd := exec.Command("ssh")
	prepareContainment(cmd)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.Pdeathsig != syscall.SIGKILL {
		t.Fatalf("SysProcAttr = %+v, want Pdeathsig SIGKILL", cmd.SysProcAttr)
	}
}
