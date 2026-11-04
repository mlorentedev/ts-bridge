//go:build linux || windows

package bootstrapssh

import (
	"bufio"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestContainment_KillsChildWhenParentDies is the abrupt-exit case of #376: a
// parent that contains a long-running child is killed outright, and the child
// must not outlive it.
func TestContainment_KillsChildWhenParentDies(t *testing.T) {
	//nolint:gosec // os.Args[0] and the test selector are fixed test-process inputs.
	parent := exec.Command(os.Args[0], "-test.run=^TestSSHHelperProcess$")
	parent.Env = append(os.Environ(), "TS_BRIDGE_SSH_HELPER=contain-parent")
	out, err := parent.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		_ = parent.Process.Kill()
		t.Fatalf("read the child pid: %v", err)
	}
	if cause, refused := strings.CutPrefix(strings.TrimSpace(line), "contain-error="); refused {
		_ = parent.Wait()
		// Not a skip: the runners this suite targets allow containment, so a refusal
		// here is a regression until a runner proves otherwise.
		t.Fatalf("this host refused containment: %s", cause)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "child=")))
	if err != nil {
		_ = parent.Process.Kill()
		t.Fatalf("parse child pid from %q: %v", line, err)
	}
	if processGone(pid) {
		t.Fatalf("child %d gone before the parent was killed", pid)
	}

	_ = parent.Process.Kill() // SIGKILL / TerminateProcess: no deferred cleanup runs
	_ = parent.Wait()
	deadline := time.Now().Add(5 * time.Second)
	for !processGone(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("child %d outlived its killed parent", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// runContainParent is the helper parent: it starts the "wait" helper with the
// shipped containment, prints its pid and waits to be killed.
func runContainParent() {
	//nolint:gosec // os.Args[0] and the test selector are fixed test-process inputs.
	child := exec.Command(os.Args[0], "-test.run=^TestSSHHelperProcess$")
	child.Env = append(os.Environ(), "TS_BRIDGE_SSH_HELPER=wait")
	prepareContainment(child)
	if err := child.Start(); err != nil {
		os.Exit(9)
	}
	job, err := containProcess(child)
	if err != nil {
		_ = child.Process.Kill() // do not orphan the 30s helper
		_, _ = os.Stdout.WriteString("contain-error=" + err.Error() + "\n")
		os.Exit(10)
	}
	_, _ = os.Stdout.WriteString("child=" + strconv.Itoa(child.Process.Pid) + "\n")
	time.Sleep(30 * time.Second)
	_ = job.Close()
	os.Exit(0)
}
