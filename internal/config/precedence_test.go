package config

import (
	"testing"
	"time"
)

// Precedence contract tests (QA-011, issue #181).
//
// Merge() applies flags > env > YAML > defaults across ~17 fields, each behind
// its own guard. These tables assert that contract per field and per layer.
//
// Two design rules make the assertions load-bearing:
//
//  1. Every layer gets a value that is unique AND different from the default.
//     If the winning value equalled the default, the assertion would also be
//     satisfied by "no layer applied anything", which is the failure mode these
//     tests exist to catch.
//
//  2. Any env var a case does not exercise is explicitly set to "" rather than
//     left alone. applyEnvString and friends treat "" as unset, so this makes
//     each case hermetic against leakage from the os.Setenv-based tests in
//     merge_test.go.

const (
	precedenceAuthKey = "tskey-auth-precedence"
	precedenceTarget  = "100.64.0.254:3389"
)

// layer identifies which source is expected to win a given case.
type layer int

const (
	layerDefault layer = iota
	layerYAML
	layerEnv
	layerFlag
)

func (l layer) String() string {
	switch l {
	case layerDefault:
		return "default"
	case layerYAML:
		return "yaml"
	case layerEnv:
		return "env"
	case layerFlag:
		return "flag"
	}
	return "unknown"
}

// mergePrecedence runs Merge, supplying the required fields (target, auth key)
// at a layer the case has left free so validation does not reject the input.
func mergePrecedence(t *testing.T, yaml PartialConfig, env map[string]string, flags FlagSet) Config {
	t.Helper()

	for k, v := range env {
		t.Setenv(k, v)
	}
	if env["TS_AUTHKEY"] == "" && flags.AuthKey == "" {
		t.Setenv("TS_AUTHKEY", precedenceAuthKey)
	}
	if yaml.Target == "" && env["TS_TARGET"] == "" && flags.Target == "" {
		yaml.Target = precedenceTarget
	}

	cfg, err := Merge(yaml, flags)
	if err != nil {
		t.Fatalf("Merge returned an unexpected error: %v", err)
	}
	return cfg
}

// --- String fields -----------------------------------------------------------

type stringField struct {
	name    string
	envKey  string
	def     string // value expected when no layer supplies one
	yamlVal string
	envVal  string
	flagVal string
	setYAML func(*PartialConfig, string)
	setFlag func(*FlagSet, string)
	get     func(Config) string
}

func stringFields() []stringField {
	return []stringField{
		{
			name: "Target", envKey: "TS_TARGET",
			yamlVal: "100.64.0.1:3389", envVal: "100.64.0.2:443", flagVal: "100.64.0.3:8080",
			setYAML: func(p *PartialConfig, v string) { p.Target = v },
			setFlag: func(f *FlagSet, v string) { f.Target = v },
			get:     func(c Config) string { return c.Target },
		},
		{
			name: "Hostname", envKey: "TS_HOSTNAME",
			yamlVal: "host-from-yaml", envVal: "host-from-env", flagVal: "host-from-flag",
			setYAML: func(p *PartialConfig, v string) { p.Hostname = v },
			setFlag: func(f *FlagSet, v string) { f.Hostname = v },
			get:     func(c Config) string { return c.Hostname },
		},
		{
			name: "ControlURL", envKey: "TS_CONTROL_URL", def: "",
			yamlVal: "https://yaml.example.com", envVal: "https://env.example.com", flagVal: "https://flag.example.com",
			setYAML: func(p *PartialConfig, v string) { p.ControlURL = v },
			setFlag: func(f *FlagSet, v string) { f.ControlURL = v },
			get:     func(c Config) string { return c.ControlURL },
		},
		{
			name: "StateDir", envKey: "TS_STATE_DIR",
			yamlVal: "/tmp/ts-bridge-yaml", envVal: "/tmp/ts-bridge-env", flagVal: "/tmp/ts-bridge-flag",
			setYAML: func(p *PartialConfig, v string) { p.StateDir = v },
			setFlag: func(f *FlagSet, v string) { f.StateDir = v },
			get:     func(c Config) string { return c.StateDir },
		},
		{
			name: "HealthAddr", envKey: "TS_HEALTH_ADDR", def: "",
			yamlVal: "127.0.0.1:9001", envVal: "127.0.0.1:9002", flagVal: "127.0.0.1:9003",
			setYAML: func(p *PartialConfig, v string) { p.HealthAddr = v },
			setFlag: func(f *FlagSet, v string) { f.HealthAddr = v },
			get:     func(c Config) string { return c.HealthAddr },
		},
		{
			name: "LogFormat", envKey: "TS_LOG_FORMAT", def: "text",
			yamlVal: "json", envVal: "text", flagVal: "json",
			setYAML: func(p *PartialConfig, v string) { p.LogFormat = v },
			setFlag: func(f *FlagSet, v string) { f.LogFormat = v },
			get:     func(c Config) string { return c.LogFormat },
		},
	}
}

func TestPrecedenceStringFields(t *testing.T) {
	for _, f := range stringFields() {
		for _, want := range []layer{layerYAML, layerEnv, layerFlag} {
			t.Run(f.name+"/"+want.String()+" wins", func(t *testing.T) {
				var yaml PartialConfig
				flags := FlagSet{}
				env := map[string]string{f.envKey: ""}

				// Populate every layer up to and including the winner, so the
				// assertion proves the winner beat a real competitor rather
				// than filling a vacuum.
				f.setYAML(&yaml, f.yamlVal)
				expected := f.yamlVal
				if want >= layerEnv {
					env[f.envKey] = f.envVal
					expected = f.envVal
				}
				if want >= layerFlag {
					f.setFlag(&flags, f.flagVal)
					expected = f.flagVal
				}

				cfg := mergePrecedence(t, yaml, env, flags)
				if got := f.get(cfg); got != expected {
					t.Errorf("%s: expected %s value %q, got %q", f.name, want, expected, got)
				}
			})
		}
	}
}

// --- Duration fields ---------------------------------------------------------
//
// IdleTimeout is deliberately absent: its applyFlags guard is `>= 0`, so an
// unset flag overwrites env and YAML. That is issue #282, asserted separately
// below.

type durationField struct {
	name    string
	envKey  string
	def     time.Duration
	yamlVal time.Duration
	envVal  time.Duration
	flagVal time.Duration
	setYAML func(*PartialConfig, time.Duration)
	setFlag func(*FlagSet, time.Duration)
	get     func(Config) time.Duration
}

func durationFields() []durationField {
	return []durationField{
		{
			name: "ConnectTimeout", envKey: "TS_TIMEOUT", def: defaultTimeout,
			yamlVal: 3 * time.Minute, envVal: 2 * time.Minute, flagVal: 90 * time.Second,
			setYAML: func(p *PartialConfig, v time.Duration) { p.Timeout = v },
			setFlag: func(f *FlagSet, v time.Duration) { f.Timeout = v },
			get:     func(c Config) time.Duration { return c.ConnectTimeout },
		},
		{
			name: "DialTimeout", envKey: "TS_DIAL_TIMEOUT", def: defaultDialTimeout,
			yamlVal: 11 * time.Second, envVal: 12 * time.Second, flagVal: 13 * time.Second,
			setYAML: func(p *PartialConfig, v time.Duration) { p.DialTimeout = v },
			setFlag: func(f *FlagSet, v time.Duration) { f.DialTimeout = v },
			get:     func(c Config) time.Duration { return c.DialTimeout },
		},
		{
			name: "DrainTimeout", envKey: "TS_DRAIN_TIMEOUT", def: defaultDrainTimeout,
			yamlVal: 21 * time.Second, envVal: 22 * time.Second, flagVal: 23 * time.Second,
			setYAML: func(p *PartialConfig, v time.Duration) { p.DrainTimeout = v },
			setFlag: func(f *FlagSet, v time.Duration) { f.DrainTimeout = v },
			get:     func(c Config) time.Duration { return c.DrainTimeout },
		},
		{
			name: "DialBackoffBase", envKey: "TS_DIAL_BACKOFF_BASE", def: defaultDialBackoffBase,
			yamlVal: 31 * time.Second, envVal: 32 * time.Second, flagVal: 33 * time.Second,
			setYAML: func(p *PartialConfig, v time.Duration) { p.DialBackoffBase = v },
			setFlag: func(f *FlagSet, v time.Duration) { f.DialBackoffBase = v },
			get:     func(c Config) time.Duration { return c.DialBackoffBase },
		},
		{
			name: "DialBackoffMax", envKey: "TS_DIAL_BACKOFF_MAX", def: defaultDialBackoffMax,
			yamlVal: 41 * time.Second, envVal: 42 * time.Second, flagVal: 43 * time.Second,
			setYAML: func(p *PartialConfig, v time.Duration) { p.DialBackoffMax = v },
			setFlag: func(f *FlagSet, v time.Duration) { f.DialBackoffMax = v },
			get:     func(c Config) time.Duration { return c.DialBackoffMax },
		},
	}
}

func TestPrecedenceDurationFields(t *testing.T) {
	for _, f := range durationFields() {
		// Guard the test data itself: a layer value equal to the default would
		// make the assertion pass without the layer having applied anything.
		for _, v := range []time.Duration{f.yamlVal, f.envVal, f.flagVal} {
			if v == f.def {
				t.Fatalf("%s: test value %v equals the default; pick a distinct value", f.name, v)
			}
		}

		for _, want := range []layer{layerYAML, layerEnv, layerFlag} {
			t.Run(f.name+"/"+want.String()+" wins", func(t *testing.T) {
				var yaml PartialConfig
				flags := FlagSet{}
				env := map[string]string{f.envKey: ""}

				f.setYAML(&yaml, f.yamlVal)
				expected := f.yamlVal
				if want >= layerEnv {
					env[f.envKey] = f.envVal.String()
					expected = f.envVal
				}
				if want >= layerFlag {
					f.setFlag(&flags, f.flagVal)
					expected = f.flagVal
				}

				cfg := mergePrecedence(t, yaml, env, flags)
				if got := f.get(cfg); got != expected {
					t.Errorf("%s: expected %s value %v, got %v", f.name, want, expected, got)
				}
			})
		}
	}
}

// --- Numeric fields ----------------------------------------------------------
//
// DialRetries is deliberately absent for the same reason as IdleTimeout: see
// TestPrecedenceIssue282UnsetNumericFlagsClobber below.

func TestPrecedenceMaxConnections(t *testing.T) {
	const (
		yamlVal int64 = 111
		envVal  int64 = 222
		flagVal int64 = 333
	)

	for _, want := range []layer{layerYAML, layerEnv, layerFlag} {
		t.Run(want.String()+" wins", func(t *testing.T) {
			yaml := PartialConfig{MaxConnections: yamlVal}
			flags := FlagSet{}
			env := map[string]string{"TS_MAX_CONNECTIONS": ""}
			expected := yamlVal

			if want >= layerEnv {
				env["TS_MAX_CONNECTIONS"] = "222"
				expected = envVal
			}
			if want >= layerFlag {
				flags.MaxConns = flagVal
				expected = flagVal
			}

			cfg := mergePrecedence(t, yaml, env, flags)
			if cfg.MaxConnections != expected {
				t.Errorf("MaxConnections: expected %s value %d, got %d", want, expected, cfg.MaxConnections)
			}
		})
	}
}

// --- Defaults ----------------------------------------------------------------

func TestPrecedenceDefaultsWhenNoLayerSupplies(t *testing.T) {
	env := map[string]string{
		"TS_TIMEOUT": "", "TS_DIAL_TIMEOUT": "", "TS_DRAIN_TIMEOUT": "",
		"TS_DIAL_BACKOFF_BASE": "", "TS_DIAL_BACKOFF_MAX": "", "TS_MAX_CONNECTIONS": "",
		"TS_LOG_FORMAT": "", "TS_CONTROL_URL": "", "TS_HEALTH_ADDR": "",
	}
	cfg := mergePrecedence(t, PartialConfig{}, env, FlagSet{})

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"ConnectTimeout", cfg.ConnectTimeout, defaultTimeout},
		{"DialTimeout", cfg.DialTimeout, defaultDialTimeout},
		{"DrainTimeout", cfg.DrainTimeout, defaultDrainTimeout},
		{"DialBackoffBase", cfg.DialBackoffBase, defaultDialBackoffBase},
		{"DialBackoffMax", cfg.DialBackoffMax, defaultDialBackoffMax},
		{"MaxConnections", cfg.MaxConnections, int64(defaultMaxConnections)},
		{"LogFormat", cfg.LogFormat, "text"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: expected default %v, got %v", c.name, c.want, c.got)
		}
	}
}

// --- Zero-sentinel boundary --------------------------------------------------
//
// A zero or empty value in a higher layer does NOT override a lower one: zero
// means "unset", which is how absence is representable in a struct without
// pointers. This is deliberate contract, confirmed 2026-08-07, and it is what
// the `> 0` / `!= ""` guards in applyYAML/applyFlags encode.
//
// These cases are what kill the CONDITIONALS_BOUNDARY mutants (`> 0` -> `>= 0`);
// the unique-value cases above cannot, because they never exercise zero.

func TestZeroInHigherLayerDoesNotOverride(t *testing.T) {
	t.Run("yaml zero duration leaves the default", func(t *testing.T) {
		yaml := PartialConfig{Timeout: 0, DialTimeout: 0, DrainTimeout: 0}
		env := map[string]string{"TS_TIMEOUT": "", "TS_DIAL_TIMEOUT": "", "TS_DRAIN_TIMEOUT": ""}

		cfg := mergePrecedence(t, yaml, env, FlagSet{})
		if cfg.ConnectTimeout != defaultTimeout {
			t.Errorf("ConnectTimeout: yaml 0 must not override; expected %v, got %v", defaultTimeout, cfg.ConnectTimeout)
		}
		if cfg.DialTimeout != defaultDialTimeout {
			t.Errorf("DialTimeout: yaml 0 must not override; expected %v, got %v", defaultDialTimeout, cfg.DialTimeout)
		}
	})

	t.Run("yaml zero duration leaves an env value", func(t *testing.T) {
		yaml := PartialConfig{Timeout: 0}
		env := map[string]string{"TS_TIMEOUT": "7m"}

		cfg := mergePrecedence(t, yaml, env, FlagSet{})
		if cfg.ConnectTimeout != 7*time.Minute {
			t.Errorf("ConnectTimeout: yaml 0 must not clear the env value; expected 7m, got %v", cfg.ConnectTimeout)
		}
	})

	t.Run("empty string in a higher layer leaves the lower one", func(t *testing.T) {
		yaml := PartialConfig{Hostname: "host-from-yaml", LogFormat: "json"}
		env := map[string]string{"TS_HOSTNAME": "", "TS_LOG_FORMAT": ""}
		flags := FlagSet{Hostname: "", LogFormat: ""} // unset flags

		cfg := mergePrecedence(t, yaml, env, flags)
		if cfg.Hostname != "host-from-yaml" {
			t.Errorf("Hostname: empty env/flag must not clear the yaml value, got %q", cfg.Hostname)
		}
		if cfg.LogFormat != "json" {
			t.Errorf("LogFormat: empty env/flag must not clear the yaml value, got %q", cfg.LogFormat)
		}
	})

	t.Run("yaml zero numeric leaves the default", func(t *testing.T) {
		yaml := PartialConfig{MaxConnections: 0, DialRetries: 0}
		env := map[string]string{"TS_MAX_CONNECTIONS": "", "TS_DIAL_RETRIES": ""}

		cfg := mergePrecedence(t, yaml, env, FlagSet{})
		if cfg.MaxConnections != int64(defaultMaxConnections) {
			t.Errorf("MaxConnections: yaml 0 must not override; expected %d, got %d", defaultMaxConnections, cfg.MaxConnections)
		}
	})

	t.Run("zero flag duration leaves the env value", func(t *testing.T) {
		env := map[string]string{"TS_DRAIN_TIMEOUT": "44s"}
		flags := FlagSet{DrainTimeout: 0} // unset flag

		cfg := mergePrecedence(t, PartialConfig{}, env, flags)
		if cfg.DrainTimeout != 44*time.Second {
			t.Errorf("DrainTimeout: zero flag must not clear the env value; expected 44s, got %v", cfg.DrainTimeout)
		}
	})
}

// --- Known defect ------------------------------------------------------------

// TestPrecedenceIssue282UnsetNumericFlagsClobber pins the two fields that break
// the contract above. --dial-retries and --idle-timeout declare a cobra default
// of 0 and their applyFlags guards are `>= 0` rather than `> 0`, so an unset
// flag overwrites whatever env or YAML supplied — and also overwrites the
// built-in default of 3 retries.
//
// Skipped rather than deleted: when #282 is fixed, remove the Skip and this
// test should pass as written. QA-011 is a test-only change (see
// specs/QA-011-config-precedence/proposal.md), so the fix is not made here.
func TestPrecedenceIssue282UnsetNumericFlagsClobber(t *testing.T) {
	t.Skip("known defect: see #282 — unset --dial-retries/--idle-timeout clobber env and YAML")

	env := map[string]string{
		"TS_DIAL_RETRIES": "5",
		"TS_IDLE_TIMEOUT": "5m",
	}
	cfg := mergePrecedence(t, PartialConfig{}, env, FlagSet{})

	if cfg.DialRetries != 5 {
		t.Errorf("DialRetries: expected env value 5, got %d", cfg.DialRetries)
	}
	if cfg.IdleTimeout != 5*time.Minute {
		t.Errorf("IdleTimeout: expected env value 5m, got %v", cfg.IdleTimeout)
	}
}
