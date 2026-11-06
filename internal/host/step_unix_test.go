//go:build !windows

package host

import (
	"strings"
	"testing"
)

// The host reads match the tools' English words (`Status: inactive`, "No
// chain/target/match by that name"), which ufw and iptables translate. A
// caller's locale must not turn a read into an unrecognized one (#608).
func TestExecRunnerReadsUnderTheCLocale(t *testing.T) {
	t.Setenv("LC_ALL", "es_ES.UTF-8")
	t.Setenv("LANG", "es_ES.UTF-8")
	t.Setenv("LANGUAGE", "es")
	out, err := execRunner{}.Output("sh", "-c", `printf '%s|%s' "$LC_ALL" "$LANGUAGE"`)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "C|" {
		t.Errorf("child locale = %q, want LC_ALL=C and no LANGUAGE", got)
	}
}
