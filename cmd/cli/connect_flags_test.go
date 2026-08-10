package cmd

import (
	"testing"
	"time"
)

// flagPtr returns a pointer to v. FlagSet models "the user passed this flag" as
// a non-nil pointer for the fields where 0 is a legitimate value (#282), so the
// case table has to express ptr(0) and nil as different expectations.
func flagPtr[T any](v T) *T { return &v }

// TestCollectFlagsUnsetNumericFlags pins the production half of #282.
//
// --idle-timeout and --dial-retries both declare a cobra default of 0, and 0 is
// a meaningful value for each (it disables the feature). Reading them
// unconditionally therefore cannot distinguish "the user asked for 0" from "the
// user passed nothing", and the unset case silently overwrote env and YAML —
// including the built-in default of 3 retries. collectFlags must record them
// only when cobra reports the flag as Changed.
//
// This exercises a fresh production connect command rather than a hand-built
// replica or package-level command state.
func TestCollectFlagsUnsetNumericFlags(t *testing.T) {
	cases := []struct {
		name string
		// "" means the flag is not passed at all — the case the bug lived in.
		setIdle     string
		setRetries  string
		wantIdle    *time.Duration
		wantRetries *int
	}{
		{
			name: "unset flags stay nil",
		},
		{
			name:       "explicit zero is recorded, not read as unset",
			setIdle:    "0s",
			setRetries: "0",
			// The distinction the whole change exists for: nil here would mean
			// "--dial-retries 0" silently fell back to the default of 3.
			wantIdle:    flagPtr(time.Duration(0)),
			wantRetries: flagPtr(0),
		},
		{
			name:        "non-zero values are carried through",
			setIdle:     "90s",
			setRetries:  "7",
			wantIdle:    flagPtr(90 * time.Second),
			wantRetries: flagPtr(7),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := newConnectCmd()
			if c.setIdle != "" {
				if err := cmd.Flags().Set("idle-timeout", c.setIdle); err != nil {
					t.Fatal(err)
				}
			}
			if c.setRetries != "" {
				if err := cmd.Flags().Set("dial-retries", c.setRetries); err != nil {
					t.Fatal(err)
				}
			}

			fs := collectFlags(cmd)

			switch {
			case c.wantIdle == nil && fs.IdleTimeout != nil:
				t.Errorf("IdleTimeout: expected nil when --idle-timeout is not passed, got %v", *fs.IdleTimeout)
			case c.wantIdle != nil && fs.IdleTimeout == nil:
				t.Errorf("IdleTimeout: expected %v to be recorded, got nil", *c.wantIdle)
			case c.wantIdle != nil && *fs.IdleTimeout != *c.wantIdle:
				t.Errorf("IdleTimeout: expected %v, got %v", *c.wantIdle, *fs.IdleTimeout)
			}

			switch {
			case c.wantRetries == nil && fs.DialRetries != nil:
				t.Errorf("DialRetries: expected nil when --dial-retries is not passed, got %d", *fs.DialRetries)
			case c.wantRetries != nil && fs.DialRetries == nil:
				t.Errorf("DialRetries: expected %d to be recorded, got nil", *c.wantRetries)
			case c.wantRetries != nil && *fs.DialRetries != *c.wantRetries:
				t.Errorf("DialRetries: expected %d, got %d", *c.wantRetries, *fs.DialRetries)
			}
		})
	}
}
