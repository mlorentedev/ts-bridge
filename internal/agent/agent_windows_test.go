package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
)

var pipeSeq atomic.Uint64

// testPaths gives every test its own lock directory and pipe name, so tests
// never meet each other's agent or the user's real one.
func testPaths(t *testing.T) Paths {
	t.Helper()
	name := fmt.Sprintf("ts-bridge-agent-test-%d-%d", time.Now().UnixNano(), pipeSeq.Add(1))
	return Paths{Base: filepath.Join(shortDir(t), "b"), Run: name}
}

// The pipe's DACL holds exactly one entry, allowing the user's SID: no other
// account, group or well-known SID can open it (ADR-018 §2).
func TestPipeAdmitsOnlyTheUser(t *testing.T) {
	p := testPaths(t)
	startAgent(t, p)
	conn, err := dial(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	h := windows.Handle(conn.(interface{ Fd() uintptr }).Fd())
	sd, err := windows.GetSecurityInfo(h, windows.SE_KERNEL_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		t.Fatalf("DACL = %v, %v; want a DACL", dacl, err)
	}
	if dacl.AceCount != 1 {
		t.Fatalf("DACL has %d entries (%s), want exactly one", dacl.AceCount, sd)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
		t.Fatalf("ACE type = %d, want an allow entry", ace.Header.AceType)
	}
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)) // #nosec G103 -- reading the SID the OS laid out after the ACE header
	self, _ := currentIdentity()
	if Identity(sid.String()) != self {
		t.Fatalf("ACE SID = %s, want the user %s", sid, self)
	}
}

// A pipe name another process already owns is a squat: the agent fails and
// names it, and never moves to another name (ADR-018 §2).
func TestStartRefusesAnOwnedPipeName(t *testing.T) {
	p := testPaths(t)
	squat, err := winio.ListenPipe(pipePrefix+p.Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = squat.Close() }()

	a, err := Start(Options{Paths: p})
	if err == nil {
		_ = a.Close()
		t.Fatal("Start served on a pipe name another listener owns")
	}
	if !strings.Contains(err.Error(), p.Run) {
		t.Fatalf("Start err = %v, want it to name the pipe", err)
	}
}

func TestEndpointRefusesANameOutsideThePipeNamespace(t *testing.T) {
	for _, name := range []string{"", `a\b`, "a/b", `..\x`} {
		if _, err := endpoint(Paths{Run: name}); err == nil {
			t.Errorf("endpoint(%q) accepted a name that is not a bare pipe name", name)
		}
	}
}

func TestDefaultPipeNameCarriesTheUserSID(t *testing.T) {
	p, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	self, _ := currentIdentity()
	if p.Run != pipeNamePrefix+string(self) || !strings.HasPrefix(string(self), "S-1-") {
		t.Fatalf("Run = %q, want %s<SID>", p.Run, pipeNamePrefix)
	}
}
