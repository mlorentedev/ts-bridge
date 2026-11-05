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
func Parse(raw string) (Descriptor, error) {
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
	cp, ok := q["cp"]
	if !ok || len(cp) == 0 {
		return wrap("missing required query param cp")
	}
	controlURL, reason := controlURLFromParam(cp[0])
	if reason != "" {
		return wrap(reason)
	}

	version, reason := versionFromParam(q.Get("v"))
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
		return 0, fmt.Sprintf("invalid version %q", vs)
	}
	return v, ""
}

// redactDescriptor masks the user info of the descriptor and of its cp control
// URL. A value url.Parse cannot read is replaced rather than echoed, because
// what it could not split may still hold a credential.
func redactDescriptor(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" {
		return "(unparseable)"
	}
	if u.User != nil {
		u.User = url.User("redacted")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		// u.Query() drops the pair it cannot decode, which can be the cp
		// holding the credential, and the raw query would then be echoed.
		return "(unparseable)"
	}
	if cp := q.Get("cp"); cp != "" && !strings.EqualFold(cp, cpSaas) {
		q.Set("cp", controlurl.Redact(cp))
		u.RawQuery = q.Encode()
	}
	return u.String()
}
