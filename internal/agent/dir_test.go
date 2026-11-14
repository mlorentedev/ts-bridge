package agent

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestDefaultDir(t *testing.T) {
	cache := filepath.Join(string(filepath.Separator), "cache")
	runtime := filepath.Join(string(filepath.Separator), "run", "user", "1000")
	cacheOK := func() (string, error) { return cache, nil }
	cacheFails := func() (string, error) { return "", errors.New("no home") }
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	tests := []struct {
		name     string
		goos     string
		env      map[string]string
		cacheDir func() (string, error)
		want     string
		wantErr  bool
	}{
		{"linux uses the runtime dir", "linux", map[string]string{"XDG_RUNTIME_DIR": runtime}, cacheOK, filepath.Join(runtime, "ts-bridge"), false},
		{"linux without a runtime dir", "linux", nil, cacheOK, filepath.Join(cache, "ts-bridge"), false},
		{"linux ignores a relative runtime dir", "linux", map[string]string{"XDG_RUNTIME_DIR": "run/user"}, cacheOK, filepath.Join(cache, "ts-bridge"), false},
		{"darwin ignores the runtime dir", "darwin", map[string]string{"XDG_RUNTIME_DIR": runtime}, cacheOK, filepath.Join(cache, "ts-bridge"), false},
		{"windows uses the cache dir", "windows", nil, cacheOK, filepath.Join(cache, "ts-bridge"), false},
		{"no cache dir", "darwin", nil, cacheFails, "", true},
		{"linux with a runtime dir needs no cache dir", "linux", map[string]string{"XDG_RUNTIME_DIR": runtime}, cacheFails, filepath.Join(runtime, "ts-bridge"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := defaultDir(tc.goos, env(tc.env), tc.cacheDir)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("defaultDir = %q, %v; want %q (error %v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
