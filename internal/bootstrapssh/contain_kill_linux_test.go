package bootstrapssh

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// processGone reports a pid that has exited: absent, or a zombie nobody reaped.
func processGone(t testing.TB, pid int) bool {
	t.Helper()
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	if err != nil {
		t.Fatalf("read pid %d state: %v", pid, err)
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
