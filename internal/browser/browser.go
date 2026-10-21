package browser

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LaunchConfig struct {
	UserDataDir    string
	PACURL         string
	StartURL       string
	AllowedOrigins []string
}

func BuildPAC(proxyAddr string, routes map[string]string) (string, error) {
	if len(routes) == 0 {
		return "", fmt.Errorf("browser route allow-list must not be empty")
	}
	if err := validateLoopbackAddress(proxyAddr); err != nil {
		return "", err
	}
	origins, err := AllowedOrigins(routes)
	if err != nil {
		return "", err
	}

	var conditions []string
	for _, origin := range origins {
		conditions = append(conditions, "origin === "+strconv.Quote(origin))
	}
	return "function effectivePort(url) {\n" +
		"  var match = url.match(/^https?:\\/\\/(?:\\[[^\\]]+\\]|[^\\/:]+):([0-9]+)(?:\\/|$)/i);\n" +
		"  if (match) { return match[1]; }\n" +
		"  return url.toLowerCase().indexOf(\"https://\") === 0 ? \"443\" : \"80\";\n" +
		"}\n" +
		"function FindProxyForURL(url, host) {\n" +
		"  host = host.toLowerCase();\n" +
		"  var origin = host + \":\" + effectivePort(url);\n" +
		"  if (" + strings.Join(conditions, " || ") + ") {\n" +
		"    return " + strconv.Quote("SOCKS5 "+proxyAddr) + ";\n" +
		"  }\n" +
		"  return \"DIRECT\";\n" +
		"}\n", nil
}

func EdgeArgs(cfg LaunchConfig) ([]string, error) {
	if cfg.UserDataDir == "" {
		return nil, fmt.Errorf("browser user-data directory is required")
	}
	if cfg.PACURL == "" {
		return nil, fmt.Errorf("browser PAC URL is required")
	}
	startURL, err := url.Parse(cfg.StartURL)
	if err != nil || startURL.Scheme != "https" || startURL.Hostname() == "" {
		return nil, fmt.Errorf("browser start URL must be an absolute HTTPS URL")
	}
	origins, err := normalizedOrigins(cfg.AllowedOrigins)
	if err != nil || len(origins) == 0 {
		return nil, fmt.Errorf("browser allow-list must not be empty")
	}
	startOrigin := canonicalHTTPSOrigin(startURL)
	if !containsOrigin(origins, startOrigin) {
		return nil, fmt.Errorf("browser start URL host is not in the browser allow-list")
	}
	hosts := originsToHosts(origins)

	rules := make([]string, 0, len(hosts)+1)
	for _, host := range hosts {
		rules = append(rules, "MAP "+host+" ~NOTFOUND")
	}
	rules = append(rules, "EXCLUDE localhost")
	return []string{
		"--user-data-dir=" + cfg.UserDataDir,
		"--proxy-pac-url=" + cfg.PACURL,
		"--host-resolver-rules=" + strings.Join(rules, ", "),
		"--no-first-run",
		cfg.StartURL,
	}, nil
}

// FindEdge returns explicitPath when it names a file, otherwise the first Edge
// install found for the running OS.
func FindEdge(explicitPath string) (string, error) {
	home, _ := os.UserHomeDir()
	return findEdgeIn(explicitPath, edgeSearchFor(runtime.GOOS, home, os.Getenv), exec.LookPath, isFile)
}

// edgeSearch is where Edge installs on one OS: executable names looked up on
// PATH, then absolute install paths, each in order.
type edgeSearch struct {
	names []string
	paths []string
}

// edgeSearchFor takes the OS as an input so every branch is testable on any
// runner (lesson 040). macOS installs an app bundle and puts nothing on PATH;
// Linux packages put microsoft-edge and microsoft-edge-stable on PATH.
func edgeSearchFor(goos, home string, getenv func(string) string) edgeSearch {
	switch goos {
	case "windows":
		var paths []string
		for _, root := range []string{getenv("PROGRAMFILES(X86)"), getenv("PROGRAMFILES")} {
			if root != "" {
				paths = append(paths, filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"))
			}
		}
		return edgeSearch{names: []string{"msedge"}, paths: paths}
	case "darwin":
		bundle := filepath.Join("Microsoft Edge.app", "Contents", "MacOS", "Microsoft Edge")
		paths := []string{filepath.Join("/Applications", bundle)}
		if home != "" {
			paths = append(paths, filepath.Join(home, "Applications", bundle))
		}
		return edgeSearch{paths: paths}
	default:
		return edgeSearch{names: []string{"microsoft-edge", "microsoft-edge-stable"}}
	}
}

func findEdgeIn(explicitPath string, search edgeSearch, lookPath func(string) (string, error), isFile func(string) bool) (string, error) {
	if explicitPath != "" {
		if !isFile(explicitPath) {
			return "", fmt.Errorf("Edge executable not found: %s", explicitPath)
		}
		return explicitPath, nil
	}
	for _, name := range search.names {
		if path, err := lookPath(name); err == nil {
			return path, nil
		}
	}
	for _, path := range search.paths {
		if isFile(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("Microsoft Edge executable not found; provide --edge-path")
}

func isFile(path string) bool {
	// #nosec G703 -- path is the operator's --edge-path or a fixed per-OS install location.
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func StartEdge(path string, args []string) error {
	command := exec.Command(path, args...) // #nosec G204 -- executable is explicit/validated and args are generated.
	if err := command.Start(); err != nil {
		return fmt.Errorf("start Edge: %w", err)
	}
	return nil
}

func StartPACServer(address, pac string) (*http.Server, string, error) {
	if err := validatePACListener(address); err != nil {
		return nil, "", err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, "", fmt.Errorf("bind PAC listener: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy.pac", func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		response.Header().Set("Cache-Control", "no-store")
		_, _ = response.Write([]byte(pac))
	})
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		_ = server.Serve(listener)
	}()
	return server, "http://" + listener.Addr().String() + "/proxy.pac", nil
}

func routeHosts(routes map[string]string) ([]string, error) {
	hosts := make([]string, 0, len(routes))
	for source := range routes {
		host, _, err := net.SplitHostPort(source)
		if err != nil || host == "" {
			return nil, fmt.Errorf("invalid browser route source %q", source)
		}
		hosts = append(hosts, host)
	}
	return normalizedHosts(hosts), nil
}

func AllowedHosts(routes map[string]string) ([]string, error) {
	return routeHosts(routes)
}

func AllowedOrigins(routes map[string]string) ([]string, error) {
	origins := make([]string, 0, len(routes))
	for source := range routes {
		host, port, err := net.SplitHostPort(source)
		if err != nil || host == "" {
			return nil, fmt.Errorf("invalid browser route source %q", source)
		}
		origins = append(origins, net.JoinHostPort(strings.ToLower(host), port))
	}
	sort.Strings(origins)
	return origins, nil
}

func normalizedHosts(hosts []string) []string {
	unique := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			unique[host] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for host := range unique {
		result = append(result, host)
	}
	sort.Strings(result)
	return result
}

func containsOrigin(origins []string, wanted string) bool {
	for _, origin := range origins {
		if origin == wanted {
			return true
		}
	}
	return false
}

func normalizedOrigins(origins []string) ([]string, error) {
	unique := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		host, port, err := net.SplitHostPort(origin)
		if err != nil || host == "" {
			return nil, fmt.Errorf("invalid browser origin %q", origin)
		}
		unique[net.JoinHostPort(strings.ToLower(host), port)] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for origin := range unique {
		result = append(result, origin)
	}
	sort.Strings(result)
	return result, nil
}

func canonicalHTTPSOrigin(parsed *url.URL) string {
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	return net.JoinHostPort(strings.ToLower(parsed.Hostname()), port)
}

func originsToHosts(origins []string) []string {
	hosts := make([]string, 0, len(origins))
	for _, origin := range origins {
		host, _, _ := net.SplitHostPort(origin)
		hosts = append(hosts, host)
	}
	return normalizedHosts(hosts)
}

func validateLoopbackAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid browser proxy address: %w", err)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("browser proxy must bind to loopback")
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return fmt.Errorf("invalid browser proxy port %q", port)
	}
	return nil
}

func validatePACListener(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid PAC listener address: %w", err)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("PAC listener must bind to loopback")
	}
	return nil
}
