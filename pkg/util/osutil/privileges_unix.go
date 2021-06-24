// +build !windows

package osutil

import (
	"fmt"
	"syscall"
)

// GainRoot escalates privileges to gain root access. Returns `uid` to be stored.
func GainRoot() (int, error) {
	uid := syscall.Getuid()

	if err := syscall.Setuid(0); err != nil {
		return 0, fmt.Errorf("failed to setuid 0: %w", err)
	}

	return uid, nil
}

// ReleaseRoot releases root privileges, setting `oldUID`.
func ReleaseRoot(oldUID int) error {
	return syscall.Setuid(oldUID)
}
