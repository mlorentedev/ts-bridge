package bootstrapssh

import "io"

// Containment ties the OpenSSH child to this process's lifetime where the OS
// allows it, so an abrupt exit (panic, a forced kill) does not leave ssh
// holding the SOCKS port (#376). It is best effort: a refusal never stops the
// bootstrap, which then behaves as it did without containment.
//
// Per OS: Linux sets Pdeathsig before the start; Windows assigns the started
// process to a kill-on-close Job Object; other systems have no equivalent and
// rely on the graceful shutdown alone.

// nopCloser is the containment of a system without one.
type nopCloser struct{}

func (nopCloser) Close() error { return nil }

var _ io.Closer = nopCloser{}
