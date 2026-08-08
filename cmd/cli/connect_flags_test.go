package cmd

import (
	"testing"
	"time"
)

// restoreFlag resets a connectCmd flag to its registered default after the test.
// connectCmd is package-level state shared by every test in this package, and
// pflag records "was this set" on the Flag itself, so both the value and the
// Changed bit have to be put back.
func restoreFlag(t *testing.T, name, def string) {
	t.Helper()
	t.Cleanup(func() {
		if err := connectCmd.Flags().Set(name, def); err != nil {
			t.Fatalf("restoring --%s: %v", name, err)
		}
		connectCmd.Flags().Lookup(name).Changed = false
	})
}

// TestCollectFlagsUnsetNumericFlags pins the production half of #282.
//
// --idle-timeout and --dial-retries both declare a cobra default of 0, and 0 is
// a meaningful value for each (it disables the feature). Reading them
// unconditionally therefore cannot distinguish "the user asked for 0" from "the
// user passed nothing", and the unset case silently overwrote env and YAML —
// including the built-in default of 3 retries. collectFlags must record them
// only when cobra reports the flag as Changed.
//
// This exercises connectCmd itself, with the flag registrations from init(),
// rather than a hand-built replica of it.
func TestCollectFlagsUnsetNumericFlags(t *testing.T) {
	t.Run("unset flags stay nil", func(t *testing.T) {
		fs := collectFlags(connectCmd)

		if fs.IdleTimeout != nil {
			t.Errorf("IdleTimeout: expected nil when --idle-timeout is not passed, got %v", *fs.IdleTimeout)
		}
		if fs.DialRetries != nil {
			t.Errorf("DialRetries: expected nil when --dial-retries is not passed, got %d", *fs.DialRetries)
		}
	})

	t.Run("explicit zero is recorded, not read as unset", func(t *testing.T) {
		restoreFlag(t, "idle-timeout", "0s")
		restoreFlag(t, "dial-retries", "0")
		if err := connectCmd.Flags().Set("idle-timeout", "0s"); err != nil {
			t.Fatal(err)
		}
		if err := connectCmd.Flags().Set("dial-retries", "0"); err != nil {
			t.Fatal(err)
		}

		fs := collectFlags(connectCmd)

		// The distinction this whole change exists for: a nil here would mean
		// "--dial-retries 0" silently fell back to the default of 3.
		if fs.IdleTimeout == nil {
			t.Error("IdleTimeout: --idle-timeout 0s must be recorded, got nil")
		} else if *fs.IdleTimeout != 0 {
			t.Errorf("IdleTimeout: expected 0s, got %v", *fs.IdleTimeout)
		}
		if fs.DialRetries == nil {
			t.Error("DialRetries: --dial-retries 0 must be recorded, got nil")
		} else if *fs.DialRetries != 0 {
			t.Errorf("DialRetries: expected 0, got %d", *fs.DialRetries)
		}
	})

	t.Run("non-zero values are carried through", func(t *testing.T) {
		restoreFlag(t, "idle-timeout", "0s")
		restoreFlag(t, "dial-retries", "0")
		if err := connectCmd.Flags().Set("idle-timeout", "90s"); err != nil {
			t.Fatal(err)
		}
		if err := connectCmd.Flags().Set("dial-retries", "7"); err != nil {
			t.Fatal(err)
		}

		fs := collectFlags(connectCmd)

		if fs.IdleTimeout == nil || *fs.IdleTimeout != 90*time.Second {
			t.Errorf("IdleTimeout: expected 90s, got %v", fs.IdleTimeout)
		}
		if fs.DialRetries == nil || *fs.DialRetries != 7 {
			t.Errorf("DialRetries: expected 7, got %v", fs.DialRetries)
		}
	})
}
