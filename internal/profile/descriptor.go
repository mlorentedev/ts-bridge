// Package profile implements the ts-bridge shareable connection descriptor
// (tsb:// scheme) and the minimal profile store that backs connect --profile.
package profile

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	scheme   = "tsb"
	cpSaas   = "saas"
	maxPort  = 65535
)

// Descriptor is a shareable, copy-paste connection token for a ts-bridge host.
// ControlURL is empty for Tailscale SaaS; non-empty for Headscale / custom planes.
// Version is the descriptor format version (default 1; bumped only on breaking changes).
type Descriptor struct {
	Host       string
	Port       int
	ControlURL string
	Version    int
}

// ParseError is returned by Parse when the input is not a valid descriptor.
type ParseError struct {
	Input  string
	Reason string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("invalid descriptor %q: %s", e.Input, e.Reason)
}

// String encodes d as a tsb:// URI. The version param is omitted when
// Version <= 1 to keep the common case compact.
func (d Descriptor) String() string {
	cp := cpSaas
	if d.ControlURL != "" {
		cp = d.ControlURL
	}

	q := url.Values{}
	q.Set("cp", cp)
	if d.Version > 1 {
		q.Set("v", strconv.Itoa(d.Version))
	}

	u := &url.URL{
		Scheme:   scheme,
		Host:     fmt.Sprintf("%s:%d", d.Host, d.Port),
		RawQuery: q.Encode(),
	}
	return u.String()
}

// Parse decodes a tsb:// URI into a Descriptor. Unknown query parameters are
// silently ignored (tolerant reader). Returns *ParseError on any malformation.
func Parse(raw string) (Descriptor, error) {
	wrap := func(reason string) (Descriptor, error) {
		return Descriptor{}, &ParseError{Input: raw, Reason: reason}
	}

	if raw == "" {
		return wrap("empty input")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return wrap(fmt.Sprintf("malformed URL: %v", err))
	}
	if u.Scheme != scheme {
		return wrap(fmt.Sprintf("expected scheme %q, got %q", scheme, u.Scheme))
	}

	host := u.Hostname()
	if host == "" {
		return wrap("missing host")
	}

	portStr := u.Port()
	if portStr == "" {
		return wrap("missing port")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > maxPort {
		return wrap(fmt.Sprintf("invalid port %q (must be 1-%d)", portStr, maxPort))
	}

	q := u.Query()
	cp, ok := q["cp"]
	if !ok || len(cp) == 0 {
		return wrap("missing required query param cp")
	}
	controlURL := cp[0]
	if strings.EqualFold(controlURL, cpSaas) {
		controlURL = ""
	}

	version := 1
	if vs := q.Get("v"); vs != "" {
		v, err := strconv.Atoi(vs)
		if err != nil || v < 1 {
			return wrap(fmt.Sprintf("invalid version %q", vs))
		}
		version = v
	}

	return Descriptor{
		Host:       host,
		Port:       port,
		ControlURL: controlURL,
		Version:    version,
	}, nil
}
