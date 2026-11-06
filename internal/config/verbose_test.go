package config

import "testing"

// Config.Verbose is what the console logger's level comes from. Only the flag
// and TS_VERBOSE can set it, and neither reached Merge once the env-only
// LoadConfig path was retired (#656).
func TestMergeVerbose(t *testing.T) {
	tests := []struct {
		name string
		env  string
		flag *bool
		want bool
	}{
		{"unset everywhere", "", nil, false},
		{"environment on", "1", nil, true},
		{"environment spelled true", "true", nil, true},
		{"environment off", "0", nil, false},
		{"flag on", "", ptr(true), true},
		{"flag on over environment off", "0", ptr(true), true},
		{"flag off over environment on", "1", ptr(false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mergePrecedence(t, PartialConfig{}, map[string]string{"TS_VERBOSE": tt.env}, FlagSet{Verbose: tt.flag})
			if cfg.Verbose != tt.want {
				t.Errorf("Verbose = %v, want %v", cfg.Verbose, tt.want)
			}
		})
	}
}
