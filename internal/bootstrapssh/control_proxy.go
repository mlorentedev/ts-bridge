package bootstrapssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

func startControlProxy(ctx context.Context, controlURL, socksAddr string) (*url.URL, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, fmt.Errorf("start bootstrap HTTP proxy: %w", err)
	}
	hostname, err := controlHostname(controlURL)
	if err != nil {
		return nil, nil, err
	}
	dialer, err := bootstrapSOCKSDialer(socksAddr)
	if err != nil {
		return nil, nil, err
	}
	transport := &http.Transport{DialContext: dialer.DialContext}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("bind bootstrap HTTP proxy: %w", err)
	}
	proxyCtx, cancel := context.WithCancel(ctx)
	server := &http.Server{
		Handler:           controlProxyHandler(proxyCtx, hostname, dialer, transport),
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return proxyCtx },
	}
	closeProxy := func() {
		cancel()
		_ = listener.Close()
		_ = server.Close()
		transport.CloseIdleConnections()
	}
	stop := context.AfterFunc(ctx, closeProxy)
	go serveControlProxy(server, listener, stop, closeProxy)
	return &url.URL{Scheme: "http", Host: listener.Addr().String()}, closeProxy, nil
}

func bootstrapSOCKSDialer(socksAddr string) (proxy.ContextDialer, error) {
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, new(net.Dialer))
	if err != nil {
		return nil, fmt.Errorf("configure bootstrap SOCKS dialer: %w", err)
	}
	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, errors.New("bootstrap SOCKS dialer must support context cancellation")
	}
	return contextDialer, nil
}

func serveControlProxy(server *http.Server, listener net.Listener, stop func() bool, closeProxy func()) {
	defer closeProxy()
	defer stop()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		slog.Error("SSH bootstrap HTTP proxy stopped", "error", err)
	}
}

func controlProxyHandler(ctx context.Context, hostname string, dialer proxy.ContextDialer, transport *http.Transport) http.Handler {
	forward := &httputil.ReverseProxy{
		Rewrite:   func(*httputil.ProxyRequest) {},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Warn("SSH bootstrap HTTP request failed", "target", r.URL.Host, "error", err)
			http.Error(w, "bootstrap SOCKS forwarding failed", http.StatusBadGateway)
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.URL.Hostname(), hostname) {
			slog.Warn("SSH bootstrap proxy denied destination", "target", r.URL.Host, "remote", r.RemoteAddr)
			http.Error(w, "bootstrap proxy destination denied", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodConnect {
			if err := controlProxyConnect(ctx, dialer, w, r); err != nil && ctx.Err() == nil {
				slog.Warn("SSH bootstrap CONNECT failed", "target", r.URL.Host, "error", err)
			}
			return
		}
		if r.URL.Scheme != "http" {
			http.Error(w, "bootstrap proxy requires an HTTP URL or CONNECT", http.StatusBadRequest)
			return
		}
		forward.ServeHTTP(w, r)
	})
}

func controlProxyConnect(ctx context.Context, dialer proxy.ContextDialer, w http.ResponseWriter, r *http.Request) error {
	// The caller owns the deadline: r.Context ends when the SDK client gives up or the proxy closes.
	upstream, err := dialer.DialContext(r.Context(), "tcp", r.URL.Host)
	if err != nil {
		http.Error(w, "bootstrap SOCKS connection failed", http.StatusBadGateway)
		return fmt.Errorf("dial bootstrap SOCKS destination: %w", err)
	}
	defer upstream.Close()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "bootstrap proxy requires HTTP/1 CONNECT", http.StatusHTTPVersionNotSupported)
		return errors.New("bootstrap HTTP connection cannot be hijacked")
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		return fmt.Errorf("hijack bootstrap CONNECT: %w", err)
	}
	defer client.Close()
	stop := context.AfterFunc(ctx, func() { _ = client.Close(); _ = upstream.Close() })
	defer stop()
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return fmt.Errorf("write bootstrap CONNECT response: %w", err)
	}
	if err := buffered.Flush(); err != nil {
		return fmt.Errorf("flush bootstrap CONNECT response: %w", err)
	}
	return relayControlProxy(client, upstream, buffered)
}

func relayControlProxy(client, upstream net.Conn, buffered io.Reader) error {
	done := make(chan error, 1)
	go func() {
		// A pipelined TLS ClientHello may already be in the HTTP reader's buffer.
		_, err := io.Copy(upstream, buffered)
		_ = upstream.Close()
		done <- controlProxyCopyError(err)
	}()
	_, err := io.Copy(client, upstream)
	_ = client.Close()
	_ = upstream.Close()
	return errors.Join(controlProxyCopyError(err), <-done)
}

func controlProxyCopyError(err error) error {
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
