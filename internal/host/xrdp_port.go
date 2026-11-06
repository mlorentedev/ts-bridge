package host

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	xrdpINIPath  = "/etc/xrdp/xrdp.ini"
	xrdpPortName = "xrdp port"
)

// xrdpPortValue is a [Globals] `port=` value that names one TCP port: the
// number alone, or tcp:// / tcp6:// with an address. Group 1 is everything
// before the port, so a rewrite keeps the operator's address.
var xrdpPortValue = regexp.MustCompile(`^((?:tcp6?://[^\s]*:)?)(\d+)$`)

// xrdpListener is where xrdp.ini makes xrdp listen. line is the index of the
// [Globals] `port=` line, -1 when there is none and xrdp uses its built-in
// 3389. globals is the [Globals] header's index, -1 when the file has none.
// value is the port line's raw value, for messages.
type xrdpListener struct {
	lines   []string
	line    int
	globals int
	value   string
	prefix  string
	port    int
}

// readXRDPListener parses xrdp.ini's [Globals] section. A value that is not a
// single TCP port (vsock, a list, a name) is an error: ts-bridge cannot tell
// which TCP port it opens.
func readXRDPListener(r Runner) (xrdpListener, error) {
	data, err := r.ReadFile(xrdpINIPath)
	if err != nil {
		return xrdpListener{}, err
	}
	l := xrdpListener{lines: strings.Split(string(data), "\n"), line: -1, globals: -1, port: defaultRDPPort}
	section := ""
	for i, raw := range l.lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line
			if section == "[Globals]" && l.globals < 0 {
				l.globals = i
			}
			continue
		}
		value, ok := globalsPort(section, line)
		if !ok {
			continue
		}
		l.line, l.value = i, value
		m := xrdpPortValue.FindStringSubmatch(l.value)
		if m == nil {
			return l, fmt.Errorf("%s sets port=%s, which is not one TCP port", xrdpINIPath, l.value)
		}
		l.prefix = m[1]
		l.port, _ = strconv.Atoi(m[2])
	}
	return l, nil
}

// globalsPort returns the value of a `port=` line when line sits in [Globals].
func globalsPort(section, line string) (string, bool) {
	key, value, ok := strings.Cut(line, "=")
	if section != "[Globals]" || !ok || strings.TrimSpace(key) != "port" {
		return "", false
	}
	return strings.TrimSpace(value), true
}

// withPort is xrdp.ini with [Globals] listening on port, inserting a `port=`
// line right after the [Globals] header when the file had none. The read
// refuses a file with neither, so there is always a place to write.
func (l xrdpListener) withPort(port int) []byte {
	lines := append([]string(nil), l.lines...)
	entry := fmt.Sprintf("port=%s%d", l.prefix, port)
	if l.line >= 0 {
		lines[l.line] = entry
	} else {
		lines = slices.Insert(lines, l.globals+1, entry)
	}
	return []byte(strings.Join(lines, "\n"))
}

// xrdpPortRead is the shared read half: the step and whether it is reported.
// Nothing is reported when xrdp's built-in default is both declared and in
// effect, so a stock host keeps its step list (#557).
func xrdpPortRead(r Runner, declared int) (xrdpListener, SetupStep, bool) {
	l, err := readXRDPListener(r)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if declared == defaultRDPPort {
			return l, SetupStep{}, false
		}
		return l, newStep(xrdpPortName, OutcomeFailed, fmt.Sprintf("Cannot apply RDP port %d: %s not found", declared, xrdpINIPath),
			"install xrdp, then re-run ts-bridge host setup"), true
	case err != nil:
		return l, newStep(xrdpPortName, OutcomeFailed, "Cannot read the xrdp listener: "+err.Error(),
			fmt.Sprintf("set port=%d in the [Globals] section of %s and restart xrdp", declared, xrdpINIPath)), true
	case l.port == declared:
		return l, newStep(xrdpPortName, OutcomeOK, fmt.Sprintf("xrdp listens on TCP %d", declared), ""), declared != defaultRDPPort
	case l.line < 0 && l.globals < 0:
		return l, newStep(xrdpPortName, OutcomeFailed, fmt.Sprintf("Cannot apply RDP port %d: %s has no [Globals] section", declared, xrdpINIPath),
			fmt.Sprintf("add a [Globals] section with port=%d to %s and restart xrdp", declared, xrdpINIPath)), true
	}
	return l, newStep(xrdpPortName, OutcomeChanged, fmt.Sprintf("xrdp listens on TCP %d, not the declared %d", l.port, declared),
		"run ts-bridge host setup as root"), true
}

// xrdpPortCheckStep reports whether xrdp listens on the declared RDP port.
func xrdpPortCheckStep(r Runner, declared int) (SetupStep, bool) {
	_, step, included := xrdpPortRead(r, declared)
	return step, included
}

// xrdpPortSetupStep points xrdp at the declared port and restarts it, only
// when it listens elsewhere.
func xrdpPortSetupStep(r Runner, declared int) (SetupStep, bool) {
	l, step, included := xrdpPortRead(r, declared)
	if step.Outcome != OutcomeChanged {
		return step, included
	}
	fix := fmt.Sprintf("set port=%d in the [Globals] section of %s, as root, and restart xrdp", declared, xrdpINIPath)
	if err := r.WriteFile(xrdpINIPath, l.withPort(declared)); err != nil {
		return newStep(xrdpPortName, OutcomeFailed, fmt.Sprintf("Could not write %s: %v", xrdpINIPath, err), fix), true
	}
	if err := r.Run("systemctl", "restart", "xrdp"); err != nil {
		return newStep(xrdpPortName, OutcomeFailed, fmt.Sprintf("xrdp set to TCP %d, but restarting it failed: %v", declared, err),
			"run `systemctl restart xrdp` as root"), true
	}
	return newStep(xrdpPortName, OutcomeChanged, fmt.Sprintf("xrdp listens on TCP %d (was %d); xrdp restarted", declared, l.port), ""), true
}
