package agent

import (
	"context"
	"net"
	"net/http"
)

// HTTPClient returns a client that reaches the agent at p and nothing else: it
// ignores the request's host, and every connection passes the transport's
// server check before a request is written. Use any host, e.g.
// "http://agent/v1/status".
func HTTPClient(p Paths) *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dial(ctx, p)
		},
	}}
}
