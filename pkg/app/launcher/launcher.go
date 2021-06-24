package launcher

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/cipher"

	"github.com/skycoin/skywire/pkg/app/appcommon"
	"github.com/skycoin/skywire/pkg/app/appnet"
	"github.com/skycoin/skywire/pkg/app/appserver"
	"github.com/skycoin/skywire/pkg/router"
	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/util/pathutil"
)

const (
	appsPIDFileName = "apps-pid.txt"
)

// Launcher associated errors.
var (
	ErrAppNotFound   = errors.New("app not found")
	ErrAppNotRunning = errors.New("app not running")
)

// AppConfig defines app startup parameters.
type AppConfig struct {
	Name      string       `json:"name"`
	Args      []string     `json:"args,omitempty"`
	AutoStart bool         `json:"auto_start"`
	Port      routing.Port `json:"port"`
}

// Config configures the launcher.
type Config struct {
	VisorPK    cipher.PubKey
	Apps       []AppConfig
	ServerAddr string
	BinPath    string
	LocalPath  string
}

// Launcher is responsible for launching and keeping track of app states.
type Launcher struct {
	conf  Config
	log   logrus.FieldLogger
	r     router.Router
	procM appserver.ProcManager
	apps  map[string]AppConfig
	mx    sync.Mutex
}

// NewLauncher creates a new launcher.
func NewLauncher(log logrus.FieldLogger, conf Config, dmsgC *dmsg.Client, r router.Router, procM appserver.ProcManager) (*Launcher, error) {
	launcher := &Launcher{
		conf:  conf,
		log:   log,
		r:     r,
		procM: procM,
	}

	// Ensure the existence of directories.
	if err := ensureDir(&launcher.conf.BinPath); err != nil {
		return nil, err
	}
	if err := ensureDir(&launcher.conf.LocalPath); err != nil {
		return nil, err
	}

	// Prepare networks.
	skyN := appnet.NewSkywireNetworker(log.WithField("_", appnet.TypeSkynet), r)
	if err := appnet.AddNetworker(appnet.TypeSkynet, skyN); err != nil {
		return nil, err
	}
	dmsgN := appnet.NewDMSGNetworker(dmsgC)
	if err := appnet.AddNetworker(appnet.TypeDmsg, dmsgN); err != nil {
		return nil, err
	}

	// Kill old processes.
	if err := launcher.killHangingProcesses(); err != nil {
		return nil, err
	}

	// Prepare apps (autostart if necessary).
	apps := make(map[string]AppConfig, len(conf.Apps))
	for _, ac := range conf.Apps {
		apps[ac.Name] = ac
	}
	launcher.apps = apps

	return launcher, nil
}

// ResetConfig resets the launcher config.
func (l *Launcher) ResetConfig(conf Config) {
	l.mx.Lock()
	defer l.mx.Unlock()

	apps := make(map[string]AppConfig, len(conf.Apps))
	for _, ac := range conf.Apps {
		apps[ac.Name] = ac
	}
	l.apps = apps

	// we shouldn't change directories of apps, it causes
	// all kinds of troubles and also doesn't make sense.
	// So, just changing individual fields
	l.conf.VisorPK = conf.VisorPK
	l.conf.Apps = conf.Apps
	l.conf.ServerAddr = conf.ServerAddr
}

// EnvMaker makes a list of environment variables with their values set
// It is used to let other code to decide how environment variables should be built
type EnvMaker func() ([]string, error)

// EnvMap is a mapping from application name to environment maker function
type EnvMap map[string]EnvMaker

// AutoStart auto-starts marked apps.
func (l *Launcher) AutoStart(envMap EnvMap) error {
	if envMap == nil {
		envMap = make(EnvMap)
	}
	log := l.log.WithField("func", "AutoStart")

	l.mx.Lock()
	defer l.mx.Unlock()

	for name, ac := range l.apps {
		if !ac.AutoStart {
			continue
		}
		var envs []string
		if makeEnvs, ok := envMap[name]; ok {
			var err error
			envs, err = makeEnvs()
			if err != nil {
				return fmt.Errorf("error running %s: %w", name, err)
			}
		}
		if err := l.startApp(name, ac.Args, envs); err != nil {
			log.WithError(err).
				WithField("app_name", name).
				WithField("args", ac.Args).
				WithField("envs", envs).
				Warn("Failed to start app.")
		}
	}

	return nil
}

// AppState returns a single app state of given name.
func (l *Launcher) AppState(name string) (*AppState, bool) {
	l.mx.Lock()
	defer l.mx.Unlock()

	ac, ok := l.apps[name]
	if !ok {
		return nil, false
	}
	state := &AppState{AppConfig: ac, Status: AppStatusStopped}
	if _, ok := l.procM.ProcByName(ac.Name); ok {
		state.Status = AppStatusRunning
	}
	return state, true
}

// AppStates returns list of AppStates for all registered apps.
func (l *Launcher) AppStates() []*AppState {
	l.mx.Lock()
	defer l.mx.Unlock()

	var states []*AppState
	for _, app := range l.apps {
		state := &AppState{AppConfig: app, Status: AppStatusStopped}
		if proc, ok := l.procM.ProcByName(app.Name); ok {
			state.DetailedStatus = proc.DetailedStatus()
			connSummary := proc.ConnectionsSummary()
			if connSummary != nil {
				state.Status = AppStatusRunning
			}
		}
		states = append(states, state)
	}
	return states
}

// StartApp starts cmd with given args and env.
// If 'args' is nil, default args will be used.
func (l *Launcher) StartApp(cmd string, args, envs []string) error {
	l.mx.Lock()
	defer l.mx.Unlock()

	return l.startApp(cmd, args, envs)
}

func (l *Launcher) startApp(cmd string, args, envs []string) error {
	log := l.log.WithField("func", "StartApp").WithField("cmd", cmd)

	// Obtain associated app config.
	ac, ok := l.apps[cmd]
	if !ok {
		return ErrAppNotFound
	}

	if args != nil {
		ac.Args = args
	}

	// Make proc config.
	procConf, err := makeProcConfig(l.conf, ac, envs)
	if err != nil {
		return err
	}

	// Start proc and persist pid.
	pid, err := l.procM.Start(procConf)
	if err != nil {
		return err
	}
	if err := l.persistPID(cmd, pid); err != nil {
		log.WithError(err).Warn("Failed to persist pid.")
	}

	return nil
}

// StopApp stops running app.
func (l *Launcher) StopApp(name string) (*appserver.Proc, error) {
	log := l.log.WithField("func", "StopApp").WithField("app_name", name)

	proc, ok := l.procM.ProcByName(name)
	if !ok {
		return nil, ErrAppNotRunning
	}

	l.log.Info("Stopping app...")

	if err := l.procM.Stop(name); err != nil {
		log.WithError(err).Warn("Failed to stop app.")
		return proc, err
	}

	return proc, nil
}

// RestartApp restarts a running app.
func (l *Launcher) RestartApp(name string) error {
	l.log.WithField("func", "RestartApp").WithField("app_name", name).
		Info("Restarting app...")

	proc, err := l.StopApp(name)
	if err != nil {
		return fmt.Errorf("failed to stop %s: %w", name, err)
	}

	cmd := proc.Cmd()
	if err := l.StartApp(name, nil, cmd.Env); err != nil {
		return fmt.Errorf("failed to start %s: %w", name, err)
	}

	return nil
}

func makeProcConfig(lc Config, ac AppConfig, envs []string) (appcommon.ProcConfig, error) {
	procConf := appcommon.ProcConfig{
		AppName:     ac.Name,
		AppSrvAddr:  lc.ServerAddr,
		ProcKey:     appcommon.RandProcKey(),
		ProcArgs:    ac.Args,
		ProcEnvs:    envs,
		ProcWorkDir: filepath.Join(lc.LocalPath, ac.Name),
		VisorPK:     lc.VisorPK,
		RoutingPort: ac.Port,
		BinaryLoc:   filepath.Join(lc.BinPath, ac.Name),
		LogDBLoc:    filepath.Join(lc.LocalPath, ac.Name+"_log.db"),
	}
	err := ensureDir(&procConf.ProcWorkDir)
	return procConf, err
}

func ensureDir(path *string) error {
	var err error
	if *path, err = filepath.Abs(*path); err != nil {
		return fmt.Errorf("failed to expand path: %s", err)
	}
	if _, err := os.Stat(*path); !os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(*path, 0707); err != nil {
		return fmt.Errorf("failed to create dir: %s", err)
	}
	return nil
}

/*
	<<< PID management >>>
*/

func (l *Launcher) pidFile() (*os.File, error) {
	return os.OpenFile(filepath.Join(l.conf.LocalPath, appsPIDFileName), os.O_RDWR|os.O_CREATE, 0606) //nolint:gosec
}

func (l *Launcher) persistPID(appName string, pid appcommon.ProcID) error {
	log := l.log.
		WithField("func", "persistPID").
		WithField("app_name", appName).
		WithField("pid", pid)

	pidF, err := l.pidFile()
	if err != nil {
		return err
	}
	pidFName := pidF.Name()
	log = log.WithField("pid_file", pidFName)

	if err := pidF.Close(); err != nil {
		log.WithError(err).Warn("Failed to close PID file.")
	}

	data := fmt.Sprintf("%s %d\n", appName, pid)
	if err := pathutil.AtomicAppendToFile(pidFName, []byte(data)); err != nil {
		log.WithError(err).Warn("Failed to save PID to file.")
	}

	return nil
}

func (l *Launcher) killHangingProcesses() error {
	log := l.log.WithField("func", "killHangingProcesses")

	pidF, err := l.pidFile()
	if err != nil {
		return err
	}
	filename := pidF.Name()
	log = log.WithField("pid_file", filename)

	scan := bufio.NewScanner(pidF)
	for scan.Scan() {
		appInfo := strings.Split(scan.Text(), " ")
		if len(appInfo) != 2 {
			err := errors.New("line should be: [app name] [pid]")
			log.WithError(err).Fatal("Failed parsing pid file.")
		}

		pid, err := strconv.Atoi(appInfo[1])
		if err != nil {
			log.WithError(err).Fatal("Failed parsing pid file.")
		}

		l.killHangingProc(appInfo[0], pid)
	}
	if err = pidF.Close(); err != nil {
		log.WithError(err).Error("Failed to close file")
	}

	// empty file
	if err = pathutil.AtomicWriteFile(filename, []byte{}); err != nil {
		log.WithError(err).Error("Failed to empty pid file.")
	}

	return nil
}

func (l *Launcher) killHangingProc(appName string, pid int) {
	log := l.log.WithField("app_name", appName).WithField("pid", pid)

	p, err := os.FindProcess(pid)
	if err != nil {
		if runtime.GOOS != "windows" {
			log.Info("Process not found.")
		}
		return
	}

	err = p.Signal(syscall.SIGKILL)
	if err != nil {
		return
	}
	log.Info("Killed hanging child process that ran previously with this visor.")
}
