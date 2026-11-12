package host

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	firewalldCmd = "firewall-cmd"
	// firewalldNotRunning is the exit status of `firewall-cmd --state` when
	// firewalld is stopped, beside the words "not running" (probed for #739).
	firewalldNotRunning = 252
)

// readForeignFirewall looks for a firewall the read does not parse: firewalld,
// or a native nftables ruleset hooked on input (#739). Where one filters
// inbound traffic, an ACCEPT iptables policy over an empty chain says nothing
// about the host, and a rule appended to iptables admits nothing past it. A
// probe that fails is an unknown read: found is true and reason says why.
func readForeignFirewall(r Runner, port int) (read firewallRead, found bool) {
	if read, found := readFirewalld(r, port); found {
		return read, true
	}
	return readNFTForeign(r, port)
}

func readFirewalld(r Runner, port int) (firewallRead, bool) {
	if _, err := r.LookPath(firewalldCmd); err != nil {
		return firewallRead{}, false
	}
	out, err := r.Output(firewalldCmd, "--state")
	state := strings.TrimSpace(string(out))
	switch {
	case err == nil && state == "running":
		return firewallRead{other: "firewalld is running", fix: firewalldFix(port)}, true
	case state == "not running" || exitStatus(err) == firewalldNotRunning: // the words decide; 252 is the convention
		return firewallRead{}, false
	case err == nil:
		err = fmt.Errorf("printed %q, neither running nor not running", state)
	}
	return firewallRead{reason: fmt.Sprintf("%s --state: %v%s", firewalldCmd, err, toolSaid(err)),
		fix: fmt.Sprintf("run `%s --state` as root to see why it fails", firewalldCmd)}, true
}

// firewalldFix gives tailscale0 a zone of its own, so the port opens on the
// tailnet only. --change-interface also moves an interface another zone holds,
// where --add-interface fails with ZONE_CONFLICT (probed for #739).
func firewalldFix(port int) string {
	return fmt.Sprintf("as root, give %[1]s a zone of its own and allow the port there: `firewall-cmd --permanent --new-zone=tailnet` (once), "+
		"`firewall-cmd --permanent --zone=tailnet --change-interface=%[1]s`, `firewall-cmd --permanent --zone=tailnet --add-port=%[2]d/tcp`, then `firewall-cmd --reload`",
		tailnetInterface, port)
}

func readNFTForeign(r Runner, port int) (firewallRead, bool) {
	if _, err := r.LookPath("nft"); err != nil {
		return firewallRead{}, false
	}
	out, err := r.Output("nft", "list", "chains")
	if err != nil {
		return firewallRead{reason: fmt.Sprintf("nft list chains: %v%s", err, toolSaid(err)),
			fix: "run `nft list chains` as root to see why it fails"}, true
	}
	family, table, chain := nftForeignInput(string(out))
	if table == "" {
		return firewallRead{}, false
	}
	return firewallRead{
		other: fmt.Sprintf("the nftables table %s %s filters inbound traffic (its chain %s hooks input)", family, table, chain),
		fix: fmt.Sprintf("as root, run `nft insert rule %s %s %s iifname \"%s\" tcp dport %d accept`, and add the rule to your distribution's nftables ruleset so it survives a reboot",
			family, table, chain, tailnetInterface, port),
	}, true
}

// iptNFTTables are the tables iptables-nft keeps its chains in. tailscaled's
// nftables mode and ufw on iptables-nft use them too, so a chain in one of them
// is what the iptables read already sees.
var iptNFTTables = []string{"filter", "nat", "mangle", "raw", "security"}

// nftForeignInput returns the first base chain hooked on input, in `nft list
// chains` output, whose table is not one of iptables-nft's. A table without an
// input hook filters no inbound traffic.
func nftForeignInput(out string) (family, table, chain string) {
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		switch {
		case len(f) >= 3 && f[0] == "table":
			family, table, chain = f[1], f[2], ""
		case len(f) >= 2 && f[0] == "chain":
			chain = f[1]
		case chain != "" && nftHooksInput(f) && nftForeignTable(family, table):
			return family, table, chain
		}
	}
	return "", "", ""
}

func nftHooksInput(f []string) bool {
	i := slices.Index(f, "hook")
	return i >= 0 && i+1 < len(f) && f[i+1] == "input"
}

// nftForeignTable reports a table iptables-nft does not own, in a family that
// sees the host's inbound IP traffic.
func nftForeignTable(family, table string) bool {
	switch family {
	case "inet":
		return true
	case "ip", "ip6":
		return !slices.Contains(iptNFTTables, table)
	}
	return false
}

// exitStatus is the exit status a failed command carries, or -1.
func exitStatus(err error) int {
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return -1
}

// foreignStep reports a host whose firewall ts-bridge neither reads nor
// writes (#739): check cannot verify the port, and setup leaves the rule to
// the operator, since an iptables rule would admit nothing past it.
func foreignStep(read firewallRead, port int, command string) SetupStep {
	if command == "setup" {
		return newStep(linuxFwName, OutcomeManual,
			fmt.Sprintf("Firewall rule for TCP %d not written: %s, which ts-bridge does not write to", port, read.other), read.fix)
	}
	return newStep(linuxFwName, OutcomeUnverified,
		fmt.Sprintf("Cannot verify TCP %d: %s, which ts-bridge does not read", port, read.other),
		read.fix+"; check stays unverified on this host")
}
