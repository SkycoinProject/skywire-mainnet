package restart

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"
)

var (
	// ErrAlreadyStarted is returned when Start is already called.
	ErrAlreadyStarted = errors.New("already started")
)

const (
	// DefaultCheckDelay is a default delay for checking if a new instance is started successfully.
	DefaultCheckDelay = 1 * time.Second
	extraWaitingTime  = 1 * time.Second
	exitDelay         = 100 * time.Millisecond
	shellCommand      = "/bin/sh"
	sleepCommand      = "sleep"
	commandFlag       = "-c"
	systemdPPID       = 1
	exitCodeSuccess   = 0
	exitCodeFailure   = 1
)

// Context describes data required for restarting visor.
type Context struct {
	log        logrus.FieldLogger
	cmd        *exec.Cmd
	path       string
	ppid       int
	parentPPID int
	checkDelay time.Duration
	isStarted  int32
}

// CaptureContext captures data required for restarting visor.
// Data used by CaptureContext must not be modified before,
// therefore calling CaptureContext immediately after starting executable is recommended.
func CaptureContext() *Context {
	delay := DefaultCheckDelay + extraWaitingTime
	delaySeconds := int(delay.Seconds())
	args := strings.Join(os.Args, " ")
	// TODO: Instead of sleeping, wait until process ID exists.
	shellCmd := fmt.Sprintf("%s %d; %s", sleepCommand, delaySeconds, args)
	shellArgs := []string{commandFlag, shellCmd}

	cmd := exec.Command(shellCommand, shellArgs...) // nolint:gosec

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	path := os.Args[0]

	ppid := os.Getppid()
	parentPPID := ppidByPid(ppid)

	attachTTY(cmd)

	return &Context{
		cmd:        cmd,
		path:       path,
		ppid:       ppid,
		parentPPID: parentPPID,
		checkDelay: DefaultCheckDelay,
	}
}

// RegisterLogger registers a logger instead of standard one.
func (c *Context) RegisterLogger(logger logrus.FieldLogger) {
	if c != nil {
		c.log = logger
	}
}

// SetCheckDelay sets a check delay instead of standard one.
func (c *Context) SetCheckDelay(delay time.Duration) {
	if c != nil {
		c.checkDelay = delay
	}
}

// CmdPath returns path of cmd to be run.
func (c *Context) CmdPath() string {
	return c.path
}

// ParentSystemd returns whether parent process is supervised by systemd.
func (c *Context) ParentSystemd() bool {
	return c.parentPPID == systemdPPID
}

// Systemd returns whether process is supervised by systemd.
func (c *Context) Systemd() bool {
	return c.ppid == systemdPPID
}

// Restart restarts an executable using Context.
// If the process is supervised by systemd, it lets systemd restart the process.
func (c *Context) Restart() (err error) {
	// SIGTTIN and SIGTTOU need to be ignored to make Foreground flag of syscall.SysProcAttr work.
	// https://github.com/golang/go/issues/37217
	signal.Ignore(syscall.SIGTTIN, syscall.SIGTTOU)

	if err := c.start(); err != nil {
		return err
	}

	signal.Reset()

	// Let RPC calls complete and then exit.
	go c.exitAfterDelay(exitDelay)

	return nil
}

func (c *Context) start() (err error) {
	if c.Systemd() {
		// No need to restart process if it's supervised by systemd.
		return nil
	}

	if !atomic.CompareAndSwapInt32(&c.isStarted, 0, 1) {
		return ErrAlreadyStarted
	}

	errCh := c.startExec()
	ticker := time.NewTicker(c.checkDelay)

	select {
	case err = <-errCh:
		c.errorLogger()("Failed to start new instance: %v", err)

		// Reset c.cmd on failure so it can be reused.
		c.cmd = copyCmd(c.cmd)
		atomic.StoreInt32(&c.isStarted, 0)

	case <-ticker.C:
		c.infoLogger()("New instance started successfully, exiting from the old one")
	}

	ticker.Stop()

	return err
}

func (c *Context) exitAfterDelay(delay time.Duration) {
	time.Sleep(delay)

	if c.log != nil {
		c.log.Infof("Exiting")
	}

	exitCode := exitCodeSuccess
	if c.Systemd() {
		// Make systemd restart process if Restart=on-failure.
		exitCode = exitCodeFailure
	}

	os.Exit(exitCode)
}

func copyCmd(oldCmd *exec.Cmd) *exec.Cmd {
	newCmd := exec.Command(oldCmd.Path, oldCmd.Args...) // nolint:gosec
	newCmd.Stdout = oldCmd.Stdout
	newCmd.Stdin = oldCmd.Stdin
	newCmd.Stderr = oldCmd.Stderr
	newCmd.Env = oldCmd.Env

	return newCmd
}

func (c *Context) startExec() chan error {
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)

		c.infoLogger()("Starting new instance of executable (cmd: %q)", c.cmd.String())

		if err := c.cmd.Start(); err != nil {
			errCh <- err
			return
		}

		if err := c.cmd.Wait(); err != nil {
			errCh <- err
			return
		}
	}()

	return errCh
}

func (c *Context) infoLogger() func(string, ...interface{}) {
	if c.log != nil {
		return c.log.Infof
	}

	logger := log.New(os.Stdout, "[INFO] ", log.LstdFlags)

	return logger.Printf
}

func (c *Context) errorLogger() func(string, ...interface{}) {
	if c.log != nil {
		return c.log.Errorf
	}

	logger := log.New(os.Stdout, "[ERROR] ", log.LstdFlags)

	return logger.Printf
}

func ppidByPid(pid int) int {
	parentPPID := 0

	parentProcess, err := process.NewProcess(int32(pid))
	if err == nil {
		if parPPID, err := parentProcess.Ppid(); err == nil {
			parentPPID = int(parPPID)
		}
	}

	return parentPPID
}
