package config

import "fmt"

// validateNoNegatives rejects a negative number passed as a flag or written in
// the YAML file. The merge treats 0 as "not set" for these values, so without
// this check a negative value was dropped silently and the default was used
// instead (#806). The environment keeps its own rule: an invalid TS_* value
// warns and is not applied.
func validateNoNegatives(yamlCfg PartialConfig, flags FlagSet) error {
	inputs := []struct {
		name     string
		negative bool
		value    any
	}{
		{"--timeout", flags.Timeout < 0, flags.Timeout},
		{"--dial-timeout", flags.DialTimeout < 0, flags.DialTimeout},
		{"--drain-timeout", flags.DrainTimeout < 0, flags.DrainTimeout},
		{"--dial-backoff-base", flags.DialBackoffBase < 0, flags.DialBackoffBase},
		{"--dial-backoff-max", flags.DialBackoffMax < 0, flags.DialBackoffMax},
		{"--max-conns", flags.MaxConns < 0, flags.MaxConns},
		{"timeout", yamlCfg.Timeout < 0, yamlCfg.Timeout},
		{"dial_timeout", yamlCfg.DialTimeout < 0, yamlCfg.DialTimeout},
		{"drain_timeout", yamlCfg.DrainTimeout < 0, yamlCfg.DrainTimeout},
		{"idle_timeout", yamlCfg.IdleTimeout < 0, yamlCfg.IdleTimeout},
		{"dial_retries", yamlCfg.DialRetries < 0, yamlCfg.DialRetries},
		{"dial_backoff_base", yamlCfg.DialBackoffBase < 0, yamlCfg.DialBackoffBase},
		{"dial_backoff_max", yamlCfg.DialBackoffMax < 0, yamlCfg.DialBackoffMax},
		{"max_connections", yamlCfg.MaxConnections < 0, yamlCfg.MaxConnections},
	}
	for _, in := range inputs {
		if in.negative {
			return fmt.Errorf("%s must not be negative, got: %v", in.name, in.value)
		}
	}
	return nil
}
