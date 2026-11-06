package config

import (
	"fmt"
	"net"
)

// ValidateHealthListener refuses a health listener that is not loopback unless
// the operator opted in with --health-allow-remote or TS_HEALTH_ALLOW_REMOTE.
// The endpoint serves connection counters and the readiness reason with no
// authentication, so reaching it from the network is a decision, not a default
// (#633). It applies to every layer that can supply the address (flag, env,
// YAML, profile) because it checks the merged Config.
func ValidateHealthListener(cfg Config) error {
	if cfg.HealthAddr != "" {
		if _, _, err := net.SplitHostPort(cfg.HealthAddr); err != nil {
			return fmt.Errorf("health address %q (--health-addr, TS_HEALTH_ADDR, or health_addr in the config file or profile) is not host:port, e.g. 127.0.0.1:9090: %w", cfg.HealthAddr, err)
		}
	}
	if !HealthExposed(cfg) || cfg.HealthAllowRemote {
		return nil
	}
	return fmt.Errorf("health address %q (--health-addr, TS_HEALTH_ADDR, or health_addr in the config file or profile) is not a loopback address; "+
		"the endpoint exposes /metrics counters and the readiness reason to anything that can reach it. "+
		"Bind 127.0.0.1, or pass --health-allow-remote (TS_HEALTH_ALLOW_REMOTE=1) to accept the exposure", cfg.HealthAddr)
}

// HealthExposed reports whether the configured health listener is reachable
// beyond loopback. Only a literal loopback IP or "localhost" counts as local:
// no name is resolved, so a hostname, a wildcard, an empty host and an address
// that does not parse are all exposed.
func HealthExposed(cfg Config) bool {
	if cfg.HealthAddr == "" {
		return false
	}
	host, _, err := net.SplitHostPort(cfg.HealthAddr)
	if err != nil {
		return true
	}
	if host == "localhost" {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}
