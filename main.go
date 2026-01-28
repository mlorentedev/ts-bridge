// Package main implements a TCP bridge over Tailscale's mesh network.
// It creates an ephemeral tsnet node and forwards local connections
// to a remote target through Tailscale's encrypted tunnel.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"tailscale.com/tsnet"
)

// version is set via ldflags at build time.
var version = "dev"

// Config holds the bridge configuration.
type Config struct {
	LocalAddr      string
	Target         string
	AuthKey        string
	Hostname       string
	StateDir       string
	ConnectTimeout time.Duration
}

func main() {
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ts-bridge %s\n", version)
		os.Exit(0)
	}

	log.SetFlags(log.Ltime)

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() (Config, error) {
	target := os.Getenv("TS_TARGET")
	if target == "" {
		return Config{}, errors.New("TS_TARGET is required (format: HOST:PORT)")
	}

	// Validate target format
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return Config{}, fmt.Errorf("TS_TARGET invalid format: %w", err)
	}
	if host == "" {
		return Config{}, errors.New("TS_TARGET: host cannot be empty")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("TS_TARGET: invalid port %q", portStr)
	}

	authKey := os.Getenv("TS_AUTHKEY")
	if authKey == "" {
		return Config{}, errors.New("TS_AUTHKEY is required")
	}
	if !strings.HasPrefix(authKey, "tskey-") {
		return Config{}, errors.New("TS_AUTHKEY: invalid format (must start with tskey-)")
	}

	// Parse timeout (default 30s)
	timeout := 30 * time.Second
	if t := os.Getenv("TS_TIMEOUT"); t != "" {
		d, err := time.ParseDuration(t)
		if err != nil {
			return Config{}, fmt.Errorf("TS_TIMEOUT invalid: %w", err)
		}
		timeout = d
	}

	return Config{
		LocalAddr:      envOr("TS_LOCAL_ADDR", "127.0.0.1:33389"),
		Target:         target,
		AuthKey:        authKey,
		Hostname:       envOr("TS_HOSTNAME", "ts-bridge"),
		StateDir:       envOr("TS_STATE_DIR", "./ts-state"),
		ConnectTimeout: timeout,
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run(cfg Config) error {
	server := &tsnet.Server{
		Hostname:  cfg.Hostname,
		AuthKey:   cfg.AuthKey,
		Dir:       cfg.StateDir,
		Ephemeral: true,
		Logf:      func(string, ...any) {},
	}

	// Wait for tsnet to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	status, err := server.Up(ctx)
	if err != nil {
		return fmt.Errorf("tailscale init failed: %w", err)
	}
	log.Printf("Tailscale ready: %s", status.Self.TailscaleIPs[0])

	listener, err := net.Listen("tcp", cfg.LocalAddr)
	if err != nil {
		return fmt.Errorf("bind %s: %w", cfg.LocalAddr, err)
	}

	printBanner(cfg)

	// Graceful shutdown
	sigCtx, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	go func() {
		<-sigCtx.Done()
		log.Println("Shutting down...")
		if err := listener.Close(); err != nil {
			log.Printf("Error closing listener: %v", err)
		}
		if err := server.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
	}()

	return acceptLoop(listener, server, cfg)
}

func printBanner(cfg Config) {
	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────┐")
	fmt.Println("  │         TAILSCALE BRIDGE            │")
	fmt.Println("  ├─────────────────────────────────────┤")
	fmt.Printf("  │  Local:  %-26s │\n", cfg.LocalAddr)
	fmt.Printf("  │  Target: %-26s │\n", cfg.Target)
	fmt.Println("  └─────────────────────────────────────┘")
	fmt.Println("  Waiting for connections...")
	fmt.Println()
}

func acceptLoop(listener net.Listener, server *tsnet.Server, cfg Config) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Printf("Accept error: %v", err)
			continue
		}
		go handleConn(conn, server, cfg)
	}
}

// Buffer pool to reduce GC pressure
var bufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, 32*1024)
		return &b
	},
}

func handleConn(client net.Conn, server *tsnet.Server, cfg Config) {
	// We close connections manually when the copy loop finishes
	// defer client.Close()
	addr := client.RemoteAddr().String()

	if tcpConn, ok := client.(*net.TCPConn); ok {
		// Enable KeepAlive to prevent idle timeouts (common in RDP/SSH)
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(3 * time.Minute)
	}

	log.Printf("[+] %s connected", addr)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	remote, err := server.Dial(ctx, "tcp", cfg.Target)
	if err != nil {
		log.Printf("[-] %s dial failed: %v", addr, err)
		_ = client.Close()
		return
	}
	// defer remote.Close()

	log.Printf("[~] %s <-> %s", addr, cfg.Target)

	// Bidirectional copy
	// When one side closes, we close the other to unblock the copy.
	var once sync.Once
	closeAll := func() {
		once.Do(func() {
			_ = client.Close()
			_ = remote.Close()
		})
	}

	copyConn := func(dst, src net.Conn, direction string) {
		defer closeAll()

		bufPtr := bufferPool.Get().(*[]byte)
		defer bufferPool.Put(bufPtr)

		n, err := io.CopyBuffer(dst, src, *bufPtr)
		
		// Ignore benign close errors
		if err != nil && !errors.Is(err, net.ErrClosed) && !strings.Contains(err.Error(), "use of closed") {
			log.Printf("[!] %s %s error after %d bytes: %v", addr, direction, n, err)
		}
	}

	go copyConn(client, remote, "←")
	copyConn(remote, client, "→")

	log.Printf("[-] %s disconnected", addr)
}
