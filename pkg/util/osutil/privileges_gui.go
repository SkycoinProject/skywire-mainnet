//+build linux,systray

package osutil

import (
	"github.com/snapcore/snapd/polkit"
	"os"
	"syscall"
)

func checkAccess() bool {
	curPid := os.Getpid()
	authorization, err := polkit.CheckAuthorization(int32(curPid), 0, "", nil, 1)
	if err != nil {
		return false
	}
	return authorization
}

// GainRoot escalates privileges to gain root access. Returns `uid` to be stored.
func GainRoot() (int, error) {
	uid := syscall.Getuid()

	if err := syscall.Setuid(0); err != nil {
		if checkAccess() {
			return GainRoot()
		}
		return uid, err
	}

	return uid, nil
}

// ReleaseRoot releases root privileges, setting `oldUID`.
func ReleaseRoot(oldUID int) error {
	return syscall.Setuid(oldUID)
}
