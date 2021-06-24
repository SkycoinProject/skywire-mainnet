// +build !windows

package commands

import (
	"os/exec"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/skycoin/skywire/pkg/restart"
)

func detachProcess(delayDuration time.Duration, log logrus.FieldLogger) {
	// Versions v0.2.3 and below return 0 exit-code after update and do not trigger systemd to restart a process
	// and therefore do not support restart via systemd.
	// If --delay flag is passed, version is v0.2.3 or below.
	// Systemd has PID 1. If PPID is not 1 and PPID of parent process is 1, then
	// this process is a child process that is run after updating by a skywire-visor that is run by systemd.
	if delayDuration != 0 && !restartCtx.Systemd() && restartCtx.ParentSystemd() {
		// As skywire-visor checks if new process is run successfully in `restart.DefaultCheckDelay` after update,
		// new process should be alive after `restart.DefaultCheckDelay`.
		time.Sleep(restart.DefaultCheckDelay)

		// When a parent process exits, systemd kills child processes as well,
		// so a child process can ask systemd to restart service between after restart.DefaultCheckDelay
		// but before (restart.DefaultCheckDelay + restart.extraWaitingTime),
		// because after that time a parent process would exit and then systemd would kill its children.
		// In this case, systemd would kill both parent and child processes,
		// then restart service using an updated binary.
		cmd := exec.Command("systemctl", "restart", "skywire-visor") // nolint:gosec
		if err := cmd.Run(); err != nil {
			log.WithError(err).Errorf("Failed to restart skywire-visor service")
		} else {
			log.WithError(err).Infof("Restarted skywire-visor service")
		}

		// Detach child from parent.
		if _, err := syscall.Setsid(); err != nil {
			log.WithError(err).Errorf("Failed to call setsid()")
		}
	}

}
