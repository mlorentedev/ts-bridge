package host

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// emittedLine matches a key=value line a PowerShell read prints.
var emittedLine = regexp.MustCompile(`Write-Output "([A-Za-z0-9_]+)=`)

// scriptRunner stands in for PowerShell: it answers a script with one key=value
// line for every line the script itself prints, taking the value from values.
// Unlike a table of hand-written output, a key the script stops printing (or
// prints under another name) never reaches the parser, so the host reads it as
// an empty field, as it would on a real Windows host.
type scriptRunner struct {
	*recordingRunner
	values map[string]string
}

func (r scriptRunner) Output(name string, args ...string) ([]byte, error) {
	script := args[len(args)-1]
	var out strings.Builder
	for _, m := range emittedLine.FindAllStringSubmatch(script, -1) {
		fmt.Fprintf(&out, "%s=%s\r\n", m[1], r.values[m[1]])
	}
	return []byte(out.String()), nil
}

// TestWindowsReadsReachTheirParsers runs each key=value read's real script
// through the real parser. Before #601 the tests pasted a copy of the script
// and hand-wrote its output, so renaming port= to localport= in both left them
// green while every converged firewall rule read as drifted.
func TestWindowsReadsReachTheirParsers(t *testing.T) {
	converged := map[string]string{
		"count": "1", "enabled": "True", "action": "Allow", "direction": "Inbound",
		"profile": "Any", "remote": tailnetRemoteAddress, "protocol": "TCP", "port": "3390",
		"category": "Private", "policy": "1", "local": "0",
	}
	t.Run("firewall rule", func(t *testing.T) {
		r := scriptRunner{&recordingRunner{}, converged}
		ok, found, legacy, err := windowsFirewallRuleOK(r, "Rule", 3390)
		if err != nil || !ok || !found || legacy {
			t.Fatalf("converged rule read: ok=%v found=%v legacy=%v err=%v, want ok and found", ok, found, legacy, err)
		}
	})
	t.Run("legacy firewall rule", func(t *testing.T) {
		values := map[string]string{}
		for k, v := range converged {
			values[k] = v
		}
		values["remote"] = "Any"
		ok, _, legacy, err := windowsFirewallRuleOK(scriptRunner{&recordingRunner{}, values}, "Rule", 3390)
		if err != nil || ok || !legacy {
			t.Fatalf("any-address rule read: ok=%v legacy=%v err=%v, want legacy and not ok", ok, legacy, err)
		}
	})
	t.Run("built-in group", func(t *testing.T) {
		step, ok := builtinGroupStep(scriptRunner{&recordingRunner{}, map[string]string{"total": "3", "exposed": "2"}}, SetupStep{Name: "Firewall configuration", Outcome: OutcomeOK})
		if ok || step.Outcome != OutcomeManual {
			t.Fatalf("exposed group read: ok=%v outcome=%v, want manual", ok, step.Outcome)
		}
	})
	t.Run("rdp deny", func(t *testing.T) {
		for _, tt := range []struct {
			name          string
			policy, local string
			denied, by    bool
		}{
			{"policy denies over local allow", "1", "0", true, true},
			{"policy allows over local deny", "0", "1", false, true},
			{"local denies", "", "1", true, false},
			{"local allows", "", "0", false, false},
		} {
			t.Run(tt.name, func(t *testing.T) {
				r := scriptRunner{&recordingRunner{}, map[string]string{"policy": tt.policy, "local": tt.local}}
				d, err := readRDPDeny(r)
				if err != nil {
					t.Fatal(err)
				}
				if denied, by := d.effective(); denied != tt.denied || by != tt.by {
					t.Fatalf("effective() = %v, %v; want %v, %v", denied, by, tt.denied, tt.by)
				}
			})
		}
	})
	t.Run("network profile", func(t *testing.T) {
		r := scriptRunner{&recordingRunner{}, converged}
		if step := networkProfileStep(r); step.Outcome != OutcomeOK {
			t.Fatalf("a Private profile read as %s (%s), want %s", step.Outcome, step.Message, OutcomeOK)
		}
	})
}
