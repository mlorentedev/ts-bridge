package cmd

import "errors"

// ExitError carries a process exit status other than 1. A command returns one
// when it ran correctly but has a result to signal, as `host check` does for a
// host that is not converged (ADR-017 §2).
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }

func (e *ExitError) Unwrap() error { return e.Err }

// ExitCode maps a command's error to the process exit status: 0 for nil, the
// carried code for an ExitError anywhere in the chain, 1 for anything else.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return 1
}
