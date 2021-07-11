// +build linux,arm64 windows

package stdio

import "syscall"

// DupTo duplicates old fd into the new fd
// see dup2 and dup3 system calls
func DupTo(oldfd, newfd int) error {
	return syscall.Dup3(oldfd, newfd, 0)
}
