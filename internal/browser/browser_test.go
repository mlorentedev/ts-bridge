package browser

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPACRoutesOnlyAllowlistedHosts(t *testing.T) {
	pac, err := BuildPAC("127.0.0.1:1080", map[string]string{
		"forge.example.internal:443": "apps:443",
		"docs.example.internal:443":  "apps:443",
	})
	if err != nil {
		t.Fatalf("BuildPAC returned error: %v", err)
	}
	for _, origin := range []string{"forge.example.internal:443", "docs.example.internal:443"} {
		if !strings.Contains(pac, `origin === "`+origin+`"`) {
			t.Errorf("PAC does not contain allow-listed origin %q:\n%s", origin, pac)
		}
	}
	if !strings.Contains(pac, `var origin = host + ":" + effectivePort(url)`) {
		t.Errorf("PAC does not derive the effective origin port:\n%s", pac)
	}
	if !strings.Contains(pac, `return "SOCKS5 127.0.0.1:1080"`) {
		t.Errorf("PAC does not use the loopback SOCKS5 proxy:\n%s", pac)
	}
	if !strings.Contains(pac, `return "DIRECT"`) {
		t.Errorf("PAC does not leave other traffic direct:\n%s", pac)
	}
}

func TestBuildPACRejectsEmptyRoutes(t *testing.T) {
	if _, err := BuildPAC("127.0.0.1:1080", nil); err == nil {
		t.Fatal("BuildPAC should reject an empty allow-list")
	}
}

func TestEdgeArgsUseIsolatedProfileAndRemoteDNSForAllowedHosts(t *testing.T) {
	args, err := EdgeArgs(LaunchConfig{
		UserDataDir: `C:\Temp\ts-bridge-browser`,
		PACURL:      "http://127.0.0.1:18181/proxy.pac",
		StartURL:    "https://forge.example.internal/",
		AllowedOrigins: []string{
			"forge.example.internal:443",
			"docs.example.internal:443",
		},
	})
	if err != nil {
		t.Fatalf("EdgeArgs returned error: %v", err)
	}

	joined := strings.Join(args, "\n")
	for _, want := range []string{
		`--user-data-dir=C:\Temp\ts-bridge-browser`,
		`--proxy-pac-url=http://127.0.0.1:18181/proxy.pac`,
		`--host-resolver-rules=MAP docs.example.internal ~NOTFOUND, MAP forge.example.internal ~NOTFOUND, EXCLUDE localhost`,
		"--no-first-run",
		"https://forge.example.internal/",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Edge args missing %q:\n%s", want, joined)
		}
	}
}

func TestEdgeArgsRejectsStartURLOutsideAllowlist(t *testing.T) {
	_, err := EdgeArgs(LaunchConfig{
		UserDataDir:    t.TempDir(),
		PACURL:         "http://127.0.0.1:18181/proxy.pac",
		StartURL:       "https://unlisted.example.internal/",
		AllowedOrigins: []string{"forge.example.internal:443"},
	})
	if err == nil || !strings.Contains(err.Error(), "not in the browser allow-list") {
		t.Fatalf("EdgeArgs error = %v", err)
	}
}

func TestEdgeArgsRejectsSameHostOnUnlistedPort(t *testing.T) {
	_, err := EdgeArgs(LaunchConfig{
		UserDataDir:    t.TempDir(),
		PACURL:         "http://127.0.0.1:18181/proxy.pac",
		StartURL:       "https://forge.example.internal:8443/",
		AllowedOrigins: []string{"forge.example.internal:443"},
	})
	if err == nil || !strings.Contains(err.Error(), "not in the browser allow-list") {
		t.Fatalf("EdgeArgs error = %v", err)
	}
}

func TestFindEdgeUsesExplicitExistingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "msedge.exe")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := FindEdge(path)
	if err != nil {
		t.Fatalf("FindEdge returned error: %v", err)
	}
	if got != path {
		t.Fatalf("FindEdge = %q, want %q", got, path)
	}
}

func TestFindEdgeRejectsMissingExplicitPath(t *testing.T) {
	_, err := FindEdge(filepath.Join(t.TempDir(), "missing-msedge.exe"))
	if err == nil {
		t.Fatal("FindEdge should reject a missing explicit path")
	}
}

func TestStartPACServerServesOnlyPACDocument(t *testing.T) {
	server, pacURL, err := StartPACServer("127.0.0.1:0", "function FindProxyForURL() { return \"DIRECT\"; }\n")
	if err != nil {
		t.Fatalf("StartPACServer returned error: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	response, err := http.Get(pacURL) // #nosec G107 -- test URL is a loopback listener created above.
	if err != nil {
		t.Fatalf("GET PAC URL failed: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := response.Header.Get("Content-Type"); got != "application/x-ns-proxy-autoconfig" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(string(body), "FindProxyForURL") {
		t.Fatalf("PAC response body = %q", body)
	}

	notFound, err := http.Get(strings.TrimSuffix(pacURL, "/proxy.pac") + "/other") // #nosec G107 -- loopback test server.
	if err != nil {
		t.Fatalf("GET non-PAC URL failed: %v", err)
	}
	defer notFound.Body.Close()
	if notFound.StatusCode != http.StatusNotFound {
		t.Fatalf("non-PAC status = %d, want 404", notFound.StatusCode)
	}
}
