// Package serve runs the host side of HOST-008: a persistent tailnet node that
// publishes exactly the services the host file declares.
package serve

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"ts-bridge/internal/config"
	"ts-bridge/internal/proxy"
)

// Service is one published service: its kind and the local port it forwards to.
type Service struct {
	Kind string
	Port int
}

// Node is the part of tsnet.Server that publishing needs.
type Node interface {
	Listen(network, addr string) (net.Listener, error)
}

// Published is the set of listeners a Publish call opened on the node.
type Published struct {
	services  []Service
	listeners []net.Listener
	loops     sync.WaitGroup
	conns     sync.WaitGroup
}

// Publish listens on the node for each service, on the service's port, and
// forwards every connection to 127.0.0.1:<port> on this host through dialer.
// Nothing else listens on the node, so an undeclared port refuses (AC3). If a
// listener cannot open, the ones already open are closed.
func Publish(node Node, services []Service, dialer proxy.Dialer, base config.Config, logger *slog.Logger) (*Published, error) {
	p := &Published{services: services}
	for _, s := range services {
		ln, err := node.Listen("tcp", ":"+strconv.Itoa(s.Port))
		if err != nil {
			_ = p.closeListeners()
			return nil, fmt.Errorf("publish %s on port %d: %w", s.Kind, s.Port, err)
		}
		p.listeners = append(p.listeners, ln)
	}
	for i, ln := range p.listeners {
		cfg := base
		cfg.Target = net.JoinHostPort("127.0.0.1", strconv.Itoa(services[i].Port))
		p.loops.Add(1)
		go func() {
			defer p.loops.Done()
			// A nil cancel: a failed local dial must not stop the node.
			_ = proxy.AcceptLoop(ln, dialer, cfg, &p.conns, nil, logger)
		}()
	}
	return p, nil
}

// Services returns the services this call published.
func (p *Published) Services() []Service {
	return p.services
}

// Close stops accepting on every published port and waits for the accept
// loops to return. Connections already forwarded run to their end.
func (p *Published) Close() error {
	err := p.closeListeners()
	p.loops.Wait()
	return err
}

func (p *Published) closeListeners() error {
	var errs []error
	for _, ln := range p.listeners {
		if err := ln.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// LoopbackDialer dials this host's local services.
type LoopbackDialer struct{}

// Dial connects to addr on this host.
func (LoopbackDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return new(net.Dialer).DialContext(ctx, network, addr)
}
