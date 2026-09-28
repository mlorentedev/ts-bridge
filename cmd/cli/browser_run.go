package cmd

import (
	"context"
	"fmt"

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
	defer pacServer.Shutdown(context.Background())

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
