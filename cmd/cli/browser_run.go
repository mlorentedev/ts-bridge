package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"ts-bridge/internal/browser"
	"ts-bridge/internal/config"
)

func RunBrowser(cfg config.Config, options BrowserOptions) error {
	pac, err := browser.BuildPAC(cfg.SOCKS5Addr, cfg.SOCKS5Routes)
	if err != nil {
		return err
	}
	origins, err := browser.AllowedOrigins(cfg.SOCKS5Routes)
	if err != nil {
		return err
	}
	pacServer, pacURL, err := browser.StartPACServer(options.PACAddr, pac)
	if err != nil {
		return err
	}
	defer closePACServer(pacServer)

	if options.UserDataDir == "" {
		return fmt.Errorf("no default browser profile directory could be resolved; pass --user-data-dir")
	}
	if err := browser.PrepareProfileDir(options.UserDataDir); err != nil {
		return err
	}
	edgePath, err := browser.FindEdge(options.EdgePath)
	if err != nil {
		return err
	}
	edgeArgs, err := browser.EdgeArgs(browser.LaunchConfig{
		UserDataDir:    options.UserDataDir,
		PACURL:         pacURL,
		StartURL:       options.StartURL,
		AllowedOrigins: origins,
	})
	if err != nil {
		return err
	}

	return run(cfg, func() error {
		if err := browser.StartEdge(edgePath, edgeArgs); err != nil {
			return fmt.Errorf("launch isolated browser: %w", err)
		}
		return nil
	})
}

// closePACServer stops the PAC server within a bound: a browser that holds a
// request open must not hang the exit, so a late Shutdown falls back to Close.
func closePACServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if server.Shutdown(ctx) != nil {
		_ = server.Close()
	}
}
