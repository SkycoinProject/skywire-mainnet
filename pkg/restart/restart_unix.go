// +build !windows

package restart

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// /dev/tty is an alias for the current process TTY
const ttyDevice = "/dev/tty"

func attachTTY(cmd *exec.Cmd) {
	tty, err := os.OpenFile(ttyDevice, os.O_RDWR, 0)
	if err != nil {
		// If current process TTY cannot be opened,
		// it is pointless to attach a child process to it.
		return
	}

	fd := int(tty.Fd())

	nfd, err := syscall.Dup(fd)
	if err != nil {
		panic(err)
	}

	pgid, err := childPgid()
	if err != nil {
		// If a process group ID for child process cannot be determined,
		// it is pointless to attach a child process to it.
		return
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Foreground: true,
		Setpgid:    true,
		Ctty:       nfd,
		Pgid:       pgid,
	}
}

func childPgid() (int, error) {
	pid := os.Getpid()
	ppid := os.Getppid()

	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return 0, err
	}

	ppgid, err := syscall.Getpgid(ppid)
	if err != nil {
		return 0, err
	}

	for pgid == ppgid && ppgid != 0 {
		ppid = ppidByPid(ppid)

		ppgid, err = syscall.Getpgid(ppid)
		if err != nil {
			return 0, err
		}
	}

	if pgidExists(ppgid) {
		return ppgid, nil
	}

	if pgidExists(pgid) {
		return pgid, nil
	}

	return 0, nil
}

func pgidExists(pgid int) bool {
	n, err := syscall.Getpgid(pgid)
	return err == nil && n >= 0
}

func (c *Context) ignoreSignals() {
	// SIGTTIN and SIGTTOU need to be ignored to make Foreground flag of syscall.SysProcAttr work.
	// https://github.com/golang/go/issues/37217
	signal.Ignore(syscall.SIGTTIN, syscall.SIGTTOU)
}
