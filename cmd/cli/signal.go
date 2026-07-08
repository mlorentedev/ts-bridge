package cmd

import (
	"fmt"
	"io"
	"strings"
)

// Structured lifecycle signals for programmatic callers (UX-004, issues
// #203/#204). Two line-oriented signals share a stable "TOKEN key=value"
// grammar:
//
//	READY local=127.0.0.1:16443 target=100.64.0.11:6443   (stdout, on startup)
//	ERROR reason=bad_authkey detail="invalid preauth key"  (stderr, on init failure)
//
// The reason tokens are a machine ABI: new tokens may be added, but an
// existing token never changes meaning.
const (
	reasonBadAuthKey              = "bad_authkey"
	reasonControlPlaneUnreachable = "control_plane_unreachable"
	reasonUnknown                 = "unknown"
)

// formatReadyLine builds the machine-readable "tunnel accepting" signal.
// localAddr must be the actual bound listener address (net.Listener.Addr),
// not the requested address, so auto-port mode reports the real port.
func formatReadyLine(localAddr, target string) string {
	return fmt.Sprintf("READY local=%s target=%s", localAddr, target)
}

// formatErrorLine builds the machine-readable startup-failure signal.
// detail is wrapped in literal quotes; callers that pass untrusted error
// text should escape it first via escapeSignalDetail (emitError does).
func formatErrorLine(reason, detail string) string {
	return fmt.Sprintf(`ERROR reason=%s detail="%s"`, reason, detail)
}

// escapeSignalDetail makes an arbitrary error message safe for a single
// quoted key=value field: newlines collapse to spaces, embedded double
// quotes are backslash-escaped, and the result is trimmed so the emitted
// line stays single-line and parseable.
func escapeSignalDetail(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, `"`, `\"`)
	return strings.TrimSpace(s)
}

// emitReady writes the READY line to w (stdout in production).
func emitReady(w io.Writer, localAddr, target string) {
	fmt.Fprintln(w, formatReadyLine(localAddr, target))
}

// emitError writes the ERROR line to w (stderr in production). detail is
// escaped so the line stays single-line and parseable.
func emitError(w io.Writer, reason, detail string) {
	fmt.Fprintln(w, formatErrorLine(reason, escapeSignalDetail(detail)))
}
