//go:build !linux && !windows

package bootstrapssh

import "os"

// runContainParent has no containment to exercise on this OS (#376).
func runContainParent() { os.Exit(11) }
