package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"ts-bridge/internal/config"
)

func TestBrowserCommandResolvesBridgeAndLaunchOptions(t *testing.T) {
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_CONTROL_URL", "")

	keyFile := filepath.Join(t.TempDir(), "authkey")
	if err := os.WriteFile(keyFile, []byte("tskey-auth-test123"), 0600); err != nil {
		t.Fatal(err)
	}
	userDataDir := filepath.Join(t.TempDir(), "edge-profile")

	var capturedConfig config.Config
	var capturedOptions BrowserOptions
	loggerInitialized := false
	LoggerInit = func(config.Config) {
		loggerInitialized = true
	}
	t.Cleanup(func() { LoggerInit = nil })
	BrowserRunner = func(cfg config.Config, options BrowserOptions) error {
		capturedConfig = cfg
		capturedOptions = options
		return nil
	}
	t.Cleanup(func() { BrowserRunner = nil })

	command := newBrowserCmd()
	command.SetArgs([]string{
		"--url", "https://forge.example.internal/",
		"--socks5", "127.0.0.1:1080",
		"--route", "forge.example.internal:443=apps:443",
		"--auth-key-file", keyFile,
		"--user-data-dir", userDataDir,
	})
	if err := command.Execute(); err != nil {
		t.Fatalf("browser command returned error: %v", err)
	}

	if capturedConfig.SOCKS5Addr != "127.0.0.1:1080" {
		t.Fatalf("SOCKS5Addr = %q", capturedConfig.SOCKS5Addr)
	}
	if capturedConfig.SOCKS5Routes["forge.example.internal:443"] != "apps:443" {
		t.Fatalf("SOCKS5Routes = %#v", capturedConfig.SOCKS5Routes)
	}
	if capturedConfig.AuthKey != "tskey-auth-test123" {
		t.Fatal("auth key file was not resolved")
	}
	if capturedOptions.StartURL != "https://forge.example.internal/" {
		t.Fatalf("StartURL = %q", capturedOptions.StartURL)
	}
	if capturedOptions.UserDataDir != userDataDir {
		t.Fatalf("UserDataDir = %q, want %q", capturedOptions.UserDataDir, userDataDir)
	}
	if !loggerInitialized {
		t.Fatal("browser command did not initialize logging before the runner")
	}
}

func TestBrowserCommandPreservesYAMLSOCKS5Config(t *testing.T) {
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_CONTROL_URL", "")

	keyFile := filepath.Join(t.TempDir(), "authkey")
	if err := os.WriteFile(keyFile, []byte("tskey-auth-test123"), 0600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "browser.yml")
	configBody := "version: 1\n" +
		"socks5_addr: 127.0.0.1:1090\n" +
		"socks5_routes:\n" +
		"  \"forge.example.internal:443\": \"apps:443\"\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0600); err != nil {
		t.Fatal(err)
	}

	var captured config.Config
	BrowserRunner = func(cfg config.Config, _ BrowserOptions) error {
		captured = cfg
		return nil
	}
	t.Cleanup(func() { BrowserRunner = nil })

	command := newBrowserCmd()
	command.SetArgs([]string{
		"--config", configPath,
		"--url", "https://forge.example.internal/",
		"--auth-key-file", keyFile,
	})
	if err := command.Execute(); err != nil {
		t.Fatalf("browser command returned error: %v", err)
	}
	if captured.SOCKS5Addr != "127.0.0.1:1090" {
		t.Fatalf("SOCKS5Addr = %q, want YAML value 127.0.0.1:1090", captured.SOCKS5Addr)
	}
	if captured.SOCKS5Routes["forge.example.internal:443"] != "apps:443" {
		t.Fatalf("SOCKS5Routes = %#v, want YAML route", captured.SOCKS5Routes)
	}
}

func TestBrowserCommandRequiresAuthKeyFile(t *testing.T) {
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_AUTHKEY", "tskey-auth-from-env")
	t.Setenv("TS_CONTROL_URL", "")

	command := newBrowserCmd()
	command.SetArgs([]string{
		"--url", "https://forge.example.internal/",
		"--route", "forge.example.internal:443=apps:443",
	})
	err := command.Execute()
	if err == nil || err.Error() != "browser requires --auth-key-file or --profile" {
		t.Fatalf("browser error = %v", err)
	}
}
