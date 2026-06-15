package envfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_nonExistent(t *testing.T) {
	if err := Load("/tmp/this-file-definitely-does-not-exist-abc123.env"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_emptyFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(tmp, []byte(""), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_commentsAndBlanks(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), ".env")
	content := "# This is a comment\n\n# Another comment\n\n"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_basic(t *testing.T) {
	defer os.Unsetenv("TEST_KEY_A")
	defer os.Unsetenv("TEST_KEY_B")
	defer os.Unsetenv("TEST_KEY_C")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "TEST_KEY_A=value_a\nTEST_KEY_B=value with spaces\nTEST_KEY_C="
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_KEY_A"); v != "value_a" {
		t.Errorf("TEST_KEY_A = %q, want %q", v, "value_a")
	}
	if v := os.Getenv("TEST_KEY_B"); v != "value with spaces" {
		t.Errorf("TEST_KEY_B = %q, want %q", v, "value with spaces")
	}
	if v := os.Getenv("TEST_KEY_C"); v != "" {
		t.Errorf("TEST_KEY_C = %q, want empty", v)
	}
}

func TestLoad_quotedValues(t *testing.T) {
	defer os.Unsetenv("TEST_Q1")
	defer os.Unsetenv("TEST_Q2")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := `TEST_Q1="hello world"
TEST_Q2='single quotes'
`
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_Q1"); v != "hello world" {
		t.Errorf("TEST_Q1 = %q, want %q", v, "hello world")
	}
	if v := os.Getenv("TEST_Q2"); v != "single quotes" {
		t.Errorf("TEST_Q2 = %q, want %q", v, "single quotes")
	}
}

func TestLoad_valueWithEquals(t *testing.T) {
	defer os.Unsetenv("TEST_EQ")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "TEST_EQ=a=b=c"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_EQ"); v != "a=b=c" {
		t.Errorf("TEST_EQ = %q, want %q", v, "a=b=c")
	}
}

func TestLoad_overridesExistingEnv(t *testing.T) {
	os.Setenv("TEST_OVERRIDE", "original")
	defer os.Unsetenv("TEST_OVERRIDE")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "TEST_OVERRIDE=from_envfile"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_OVERRIDE"); v != "from_envfile" {
		t.Errorf("TEST_OVERRIDE = %q, want %q", v, "from_envfile")
	}
}

func TestLoad_idempotent(t *testing.T) {
	defer os.Unsetenv("TEST_IDEM")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "TEST_IDEM=first"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("first load error: %v", err)
	}
	if err := Load(tmp); err != nil {
		t.Fatalf("second load error: %v", err)
	}

	if v := os.Getenv("TEST_IDEM"); v != "first" {
		t.Errorf("TEST_IDEM = %q, want %q", v, "first")
	}
}

func TestLoad_skipsMalformedLines(t *testing.T) {
	defer os.Unsetenv("TEST_GOOD")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "no_equals_sign\nTEST_GOOD=valid\n=empty_key\n"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_GOOD"); v != "valid" {
		t.Errorf("TEST_GOOD = %q, want %q", v, "valid")
	}
}

func TestLoad_realWorld(t *testing.T) {
	cleanup := func() {
		for _, k := range []string{"TS_AUTHKEY", "TS_TARGET", "TS_INSTANCE_NAME"} {
			os.Unsetenv(k)
		}
	}
	cleanup()
	defer cleanup()

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "# ts-bridge environment configuration\nTS_AUTHKEY=tskey-auth-test-12345\nTS_TARGET=100.64.0.1:3389\nTS_INSTANCE_NAME=my-desktop\n"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TS_AUTHKEY"); v != "tskey-auth-test-12345" {
		t.Errorf("TS_AUTHKEY = %q, want %q", v, "tskey-auth-test-12345")
	}
	if v := os.Getenv("TS_TARGET"); v != "100.64.0.1:3389" {
		t.Errorf("TS_TARGET = %q, want %q", v, "100.64.0.1:3389")
	}
	if v := os.Getenv("TS_INSTANCE_NAME"); v != "my-desktop" {
		t.Errorf("TS_INSTANCE_NAME = %q, want %q", v, "my-desktop")
	}
}

func TestLoad_precedence_chain(t *testing.T) {
	os.Setenv("TS_TARGET", "from_os")
	defer os.Unsetenv("TS_TARGET")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "TS_TARGET=from_envfile"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TS_TARGET"); v != "from_envfile" {
		t.Errorf("after Load: TS_TARGET = %q, want %q", v, "from_envfile")
	}
}

func TestLoad_whitespaceOnlyKey(t *testing.T) {
	defer os.Unsetenv("TEST_WS")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "   TEST_WS = spaced   "
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_WS"); v != "spaced" {
		t.Errorf("TEST_WS = %q, want %q", v, "spaced")
	}
}

func TestLoad_noTrailingNewline(t *testing.T) {
	defer os.Unsetenv("TEST_NT")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "TEST_NT=no_newline"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_NT"); v != "no_newline" {
		t.Errorf("TEST_NT = %q, want %q", v, "no_newline")
	}
}

func TestLoad_mixedLineEndings(t *testing.T) {
	defer os.Unsetenv("TEST_MLE_A")
	defer os.Unsetenv("TEST_MLE_B")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "TEST_MLE_A=value_a\r\nTEST_MLE_B=value_b\n"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_MLE_A"); v != "value_a" {
		t.Errorf("TEST_MLE_A = %q, want %q", v, "value_a")
	}
	if v := os.Getenv("TEST_MLE_B"); v != "value_b" {
		t.Errorf("TEST_MLE_B = %q, want %q", v, "value_b")
	}
}

func TestLoad_exportPrefix(t *testing.T) {
	defer os.Unsetenv("TEST_EXP")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "export TEST_EXP=value"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_EXP"); v != "" {
		t.Logf("TEST_EXP = %q (export prefix not supported — intentional)", v)
	}
}

func BenchmarkLoad(b *testing.B) {
	tmp := filepath.Join(b.TempDir(), ".env")
	content := "TS_AUTHKEY=tskey-auth-xxx\nTS_TARGET=100.64.0.1:3389\nTS_INSTANCE_NAME=my-desktop\nTS_LOCAL_ADDR=127.0.0.1:33389\nTS_HEALTH_ADDR=127.0.0.1:9091\n"
	os.WriteFile(tmp, []byte(content), 0600)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Load(tmp)
	}
}

func TestLoad_emptyValueWithQuotes(t *testing.T) {
	defer os.Unsetenv("TEST_EQV")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := `TEST_EQV=""`
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_EQV"); v != "" {
		t.Errorf("TEST_EQV = %q, want empty", v)
	}
}

func TestLoad_singleCharKey(t *testing.T) {
	defer os.Unsetenv("X")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "X=val"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("X"); v != "val" {
		t.Errorf("X = %q, want %q", v, "val")
	}
}

func TestLoad_keyOnlyNoValue(t *testing.T) {
	defer os.Unsetenv("TEST_KEYONLY")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "TEST_KEYONLY"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_KEYONLY"); v != "" {
		t.Errorf("TEST_KEYONLY = %q, want empty (line has no =)", v)
	}
}

func TestLoad_unbalancedQuotes(t *testing.T) {
	defer os.Unsetenv("TEST_UQ")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := `TEST_UQ="unbalanced`
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_UQ"); v != `"unbalanced` {
		t.Errorf("TEST_UQ = %q, want %q", v, `"unbalanced`)
	}
}

func TestLoad_unicodeValue(t *testing.T) {
	defer os.Unsetenv("TEST_UNICODE")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "TEST_UNICODE=hola mundo 你好"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_UNICODE"); v != "hola mundo 你好" {
		t.Errorf("TEST_UNICODE = %q, want %q", v, "hola mundo 你好")
	}
}

func TestLoad_duplicateKeys(t *testing.T) {
	defer os.Unsetenv("TEST_DUP")

	tmp := filepath.Join(t.TempDir(), ".env")
	content := "TEST_DUP=first\nTEST_DUP=second"
	os.WriteFile(tmp, []byte(content), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := os.Getenv("TEST_DUP"); v != "second" {
		t.Errorf("TEST_DUP = %q, want %q", v, "second")
	}
}

func TestLoad_largeFile(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString(fmt.Sprintf("TEST_VAR_%04d=value_%d\n", i, i))
	}

	tmp := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(tmp, []byte(sb.String()), 0600)

	if err := Load(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	check := func(idx int, want string) {
		key := fmt.Sprintf("TEST_VAR_%04d", idx)
		if v := os.Getenv(key); v != want {
			t.Errorf("%s = %q, want %q", key, v, want)
		}
	}
	check(0, "value_0")
	check(500, "value_500")
	check(999, "value_999")
}
