// Package controlurl holds the one validation of a control-plane URL, shared by
// every place that accepts one: connect (internal/config), the host file
// (internal/host) and the tsb:// descriptor (internal/profile).
//
// It imports nothing else from ts-bridge, so the host package, which runs
// elevated and must not import the per-user stores (ADR-017 C9), can use it.
package controlurl

import (
	"errors"
	"net/url"
	"strconv"
)

// The reasons Validate gives. None carries the input: a control URL can hold
// user info, and the error ends up in a banner, a log line or the JSON log file.
var (
	ErrNotURL      = errors.New("not a valid URL")
	ErrCredentials = errors.New("must not carry credentials")
	ErrScheme      = errors.New("must be an http or https URL with a host")
	ErrPort        = errors.New("port must be between 1 and 65535")
)

// Validate reports whether raw is a control URL: an http or https URL with a
// host, no user info and a port within range. Paths and trailing slashes are
// allowed, and so is http, since a Headscale server may sit on a private net.
//
// The returned error never contains raw. It is also not the url.Error that
// url.Parse returns, which quotes the whole input.
func Validate(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return ErrNotURL
	}
	if u.User != nil {
		return ErrCredentials
	}
	// An opaque or scheme-less value can hide user info from u.User, which is
	// why it is rejected here rather than echoed or inspected.
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return ErrScheme
	}
	if !validPort(u.Port()) {
		return ErrPort
	}
	return nil
}

// validPort reports whether p, a URL's port ("" when it has none), is absent or
// within 1-65535. url.Parse checks only that it is digits.
func validPort(p string) bool {
	if p == "" {
		return true
	}
	n, err := strconv.Atoi(p)
	return err == nil && n >= 1 && n <= 65535
}
