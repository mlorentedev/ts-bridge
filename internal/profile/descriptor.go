// Package profile implements the ts-bridge shareable connection descriptor
// (tsb:// scheme) and the minimal profile store that backs connect --profile.
package profile

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"ts-bridge/internal/controlurl"
)

const (
	scheme  = "tsb"
	cpSaas  = "saas"
	maxPort = 65535
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
// Input is the descriptor with any user info masked: a shared descriptor can
// carry credentials, and this error is printed and logged.
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
func Parse(raw string) (Descriptor, error) { //nolint:funlen,gocyclo // baseline #629
	wrap := func(reason string) (Descriptor, error) {
		return Descriptor{}, &ParseError{Input: redactDescriptor(raw), Reason: reason}
	}

	if raw == "" {
		return wrap("empty input")
	}

	u, err := url.Parse(raw)
	if err != nil {
		// err is a *url.Error quoting the whole input, so it is not echoed.
		return wrap("malformed URL")
	}
	if u.Scheme != scheme {
		// The scheme found is not echoed: for "user:secret@host" it is the user.
		return wrap(fmt.Sprintf("expected scheme %q", scheme))
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
	cp, present, reason := singleParam(q, "cp")
	if reason != "" {
		return wrap(reason)
	}
	if !present {
		return wrap("missing required query param cp")
	}
	controlURL, reason := controlURLFromParam(cp)
	if reason != "" {
		return wrap(reason)
	}

	vs, _, reason := singleParam(q, "v")
	if reason != "" {
		return wrap(reason)
	}
	version, reason := versionFromParam(vs)
	if reason != "" {
		return wrap(reason)
	}

	return Descriptor{
		Host:       host,
		Port:       port,
		ControlURL: controlURL,
		Version:    version,
	}, nil
}

// singleParam returns the one value of key, whether it is present, or the
// reason it is refused when it is given more than once: validating only the
// first of two values would let the second reach the profile unchecked.
func singleParam(q url.Values, key string) (value string, present bool, reason string) {
	vals := q[key]
	switch len(vals) {
	case 0:
		return "", false, ""
	case 1:
		return vals[0], true, ""
	}
	return "", true, fmt.Sprintf("query param %s given more than once", key)
}

// controlURLFromParam turns the cp value into the descriptor's control URL
// ("" for Tailscale SaaS) or, when connect and the host file would refuse it,
// the reason it is refused.
func controlURLFromParam(cp string) (controlURL, reason string) {
	if strings.EqualFold(cp, cpSaas) {
		return "", ""
	}
	if err := controlurl.Validate(cp); err != nil {
		return "", fmt.Sprintf("query param cp: %v", err)
	}
	return cp, ""
}

// versionFromParam reads the v param, defaulting to 1 when it is absent.
func versionFromParam(vs string) (version int, reason string) {
	if vs == "" {
		return 1, ""
	}
	v, err := strconv.Atoi(vs)
	if err != nil || v < 1 {
		// vs is not echoed: it is attacker-chosen text, and %q would print it.
		return 0, "invalid version (must be a positive integer)"
	}
	return v, ""
}

// redactDescriptor reduces raw to what locates the descriptor: scheme, host
// and port. The user info is dropped, a path becomes "/…" and a query "?…",
// because credentials can sit in any parameter and escaping one parameter at a
// time is what let the next one through. A value url.Parse cannot read is
// replaced rather than echoed, since what it could not split may hold one too.
func redactDescriptor(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" {
		return "(unparseable)"
	}
	out := (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
	if u.Path != "" && u.Path != "/" {
		out += "/\u2026" // a descriptor has no path, so one carries only leak surface
	}
	if u.RawQuery != "" || u.ForceQuery {
		out += "?\u2026"
	}
	return out
}
