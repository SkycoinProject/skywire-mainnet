package commands

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof" // nolint:gosec // https://golang.org/doc/diagnostics.html#profiling
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/profile"
	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cmdutil"
	"github.com/skycoin/skycoin/src/util/logging"
	"github.com/spf13/cobra"
	"github.com/toqueteos/webbrowser"
	"gopkg.in/yaml.v3"

	"github.com/skycoin/skywire/pkg/restart"
	"github.com/skycoin/skywire/pkg/syslog"
	"github.com/skycoin/skywire/pkg/visor"
	"github.com/skycoin/skywire/pkg/visor/logstore"
	"github.com/skycoin/skywire/pkg/visor/visorconfig"
)

var uiAssets fs.FS

var restartCtx = restart.CaptureContext()

const (
	defaultConfigName    = "skywire-config.json"
	defaultServersName   = "servers.yml"
	runtimeLogMaxEntries = 300
)

var (
	tag           string
	syslogAddr    string
	pprofMode     string
	pprofAddr     string
	confPath      string
	delay         string
	launchBrowser bool
	localServer   bool
)

func init() {
	rootCmd.Flags().StringVar(&tag, "tag", "skywire", "logging tag")
	rootCmd.Flags().StringVar(&syslogAddr, "syslog", "", "syslog server address. E.g. localhost:514")
	rootCmd.Flags().StringVarP(&pprofMode, "pprofmode", "p", "", "pprof profiling mode. Valid values: cpu, mem, mutex, block, trace, http")
	rootCmd.Flags().StringVar(&pprofAddr, "pprofaddr", "localhost:6060", "pprof http port if mode is 'http'")
	rootCmd.Flags().StringVarP(&confPath, "config", "c", "", "config file location. If the value is 'STDIN', config file will be read from stdin.")
	rootCmd.Flags().StringVar(&delay, "delay", "0ns", "start delay (deprecated)") // deprecated
	rootCmd.Flags().BoolVar(&launchBrowser, "launch-browser", false, "open hypervisor web ui (hypervisor only) with system browser")
	rootCmd.Flags().BoolVar(&localServer, "local-server", false, "force skywire-visor to connect to local server")
}

var rootCmd = &cobra.Command{
	Use:   "skywire-visor",
	Short: "Skywire visor",
	Run: func(_ *cobra.Command, args []string) {
		log := initLogger(tag, syslogAddr)
		store, hook := logstore.MakeStore(runtimeLogMaxEntries)
		log.AddHook(hook)

		delayDuration, err := time.ParseDuration(delay)
		if err != nil {
			log.WithError(err).Error("Failed to parse delay duration.")
			delayDuration = time.Duration(0)
		}

		log.WithField("delay", delayDuration).
			WithField("systemd", restartCtx.Systemd()).
			WithField("parent_systemd", restartCtx.ParentSystemd()).
			Debugf("Process info")

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

		time.Sleep(delayDuration)

		if _, err := buildinfo.Get().WriteTo(log.Out); err != nil {
			log.WithError(err).Error("Failed to output build info.")
		}

		stopPProf := initPProf(log, tag, pprofMode, pprofAddr)
		defer stopPProf()

		conf := initConfig(log, args, confPath)
		err = initAddresses(conf, log)
		if err != nil {
			log.Fatal("Servers list not avialble, both from skycoin.com and your local backup")
		}

		v, ok := visor.NewVisor(conf, restartCtx)
		if !ok {
			log.Fatal("Failed to start visor.")
		}
		v.SetLogstore(store)

		if launchBrowser {
			runBrowser(conf, log)
		}

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()

		// Wait.
		<-ctx.Done()

		if err := v.Close(); err != nil {
			log.WithError(err).Error("Visor closed with error.")
		}
	},
	Version: buildinfo.Version(),
}

// Execute executes root CLI command.
func Execute(ui embed.FS) {
	uiFS, err := fs.Sub(ui, "static")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	uiAssets = uiFS

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func initLogger(tag string, syslogAddr string) *logging.MasterLogger {
	log := logging.NewMasterLogger()

	if syslogAddr != "" {
		hook, err := syslog.SetupHook(syslogAddr, tag)
		if err != nil {
			log.WithError(err).Error("Failed to connect to the syslog daemon.")
		} else {
			log.AddHook(hook)
			log.Out = ioutil.Discard
		}
	}

	return log
}

func initPProf(log *logging.MasterLogger, tag string, profMode string, profAddr string) (stop func()) {
	var optFunc func(*profile.Profile)

	switch profMode {
	case "none", "":
	case "http":
		go func() {
			err := http.ListenAndServe(profAddr, nil)
			log.WithError(err).
				WithField("mode", profMode).
				WithField("addr", profAddr).
				Info("Stopped serving pprof on http.")
		}()
	case "cpu":
		optFunc = profile.CPUProfile
	case "mem":
		optFunc = profile.MemProfile
	case "mutex":
		optFunc = profile.MutexProfile
	case "block":
		optFunc = profile.BlockProfile
	case "trace":
		optFunc = profile.TraceProfile
	}

	if optFunc != nil {
		stop = profile.Start(profile.ProfilePath("./logs/"+tag), optFunc).Stop
	}

	if stop == nil {
		stop = func() {}
	}
	return stop
}

func initConfig(mLog *logging.MasterLogger, args []string, confPath string) *visorconfig.V1 {
	log := mLog.PackageLogger("visor:config")

	var r io.Reader

	switch confPath {
	case visorconfig.StdinName:
		log.Info("Reading config from STDIN.")
		r = os.Stdin
	case "":
		// TODO: More robust solution.
		for _, arg := range args {
			if strings.HasSuffix(arg, ".json") {
				confPath = arg
				break
			}
		}

		if confPath == "" {
			confPath = "/opt/skywire/" + defaultConfigName
		}

		fallthrough
	default:
		log.WithField("filepath", confPath).Info("Reading config from file.")
		f, err := os.Open(confPath) //nolint:gosec
		if err != nil {
			log.WithError(err).
				WithField("filepath", confPath).
				Fatal("Failed to read config file.")
		}
		defer func() {
			if err := f.Close(); err != nil {
				log.WithError(err).Error("Closing config file resulted in error.")
			}
		}()
		r = f
	}

	raw, err := ioutil.ReadAll(r)
	if err != nil {
		log.WithError(err).Fatal("Failed to read in config.")
	}

	conf, err := visorconfig.Parse(mLog, confPath, raw)
	if err != nil {
		log.WithError(err).Fatal("Failed to parse config.")
	}

	if conf.Hypervisor != nil {
		conf.Hypervisor.UIAssets = uiAssets
	}

	return conf
}

func initAddresses(conf *visorconfig.V1, mLog *logging.MasterLogger) error {
	var fetchStatus bool
	var serversListYml []byte
	var servers serversList
	var err error
	log := mLog.PackageLogger("visor:config")

	// trying to get servers list from skycoin, and save it in local
	for !fetchStatus {
		log.Info("Trying to fetch servers list from skycoin")
		resp, err := http.Get(conf.ServersListAddress)
		if err != nil {
			log.Warn("Error during fetching servers list from skycoin")
			break
		}
		defer func() {
			err := resp.Body.Close()
			if err != nil {
				log.Warn(err)
			}
		}()
		serversListYml, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Warn("Error during fetching servers list from skycoin")
			break
		}
		log.Info("Servers list fetched from skycoin")

		out, err := os.Create(defaultServersName)
		if err != nil {
			log.Warn("Cannot create backup servers list file")
		}
		defer func() {
			err := out.Close()
			if err != nil {
				log.Warn(err)
			}
		}()
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			log.Warn("Cannot save backup servers list file")
		}
		log.Info("Servers list backup saved")
		fetchStatus = true
	}
	// if servers list not reached from skycoin, use stored backup file
	for !fetchStatus {
		log.Info("Trying to fetch servers list from stored backup")
		filename := filepath.Clean(defaultServersName)
		serversListYml, err = ioutil.ReadFile(filename)
		if err != nil {
			log.Warn("Cannot find backup file in path")
			break
		}
		log.Info("Servers list fetched from backup file")
		fetchStatus = true
	}
	// update conf values
	if fetchStatus {
		err := yaml.Unmarshal(serversListYml, &servers)
		if err != nil {
			log.Fatal("Error during parsing servers list")
			return err
		}

		if conf.IsTest {
			setServersConfig(conf, log, servers.Test)
		} else if localServer || checkLocalTime() {
			setServersConfig(conf, log, servers.Local)
		} else {
			setServersConfig(conf, log, servers.Worldwide)
		}
		return nil
	}
	return err
}

func setServersConfig(conf *visorconfig.V1, log *logging.Logger, servers []serversData) {
	randomServer := ((time.Now().Unix() % 11) % int64(len(servers)))
	conf.Dmsg.Discovery = servers[randomServer].Dmsg
	conf.Transport.AddressResolver = servers[randomServer].AddressResolver
	conf.Transport.Discovery = servers[randomServer].Transport
	conf.Routing.RouteFinder = servers[randomServer].Routing
	conf.UptimeTracker.Addr = servers[randomServer].UptimeTracker
	conf.Launcher.Discovery.ServiceDisc = servers[randomServer].Launcher
	log.Infof("The %s selected", servers[randomServer].Name)
}

func checkLocalTime() bool {
	return time.Now().UTC().Add(8*time.Hour).Format("2006-01-02 15:04:05 Mon") == time.Now().Local().Format("2006-01-02 15:04:05 Mon")
}

func runBrowser(conf *visorconfig.V1, log *logging.MasterLogger) {
	if conf.Hypervisor == nil {
		log.Errorln("Cannot start browser with a regular visor")
		return
	}
	addr := conf.Hypervisor.HTTPAddr
	if addr[0] == ':' {
		addr = "localhost" + addr
	}
	if addr[:4] != "http" {
		if conf.Hypervisor.EnableTLS {
			addr = "https://" + addr
		} else {
			addr = "http://" + addr
		}
	}
	go func() {
		if !checkHvIsRunning(addr, 5) {
			log.Error("Cannot open hypervisor in browser: status check failed")
			return
		}
		if err := webbrowser.Open(addr); err != nil {
			log.WithError(err).Error("webbrowser.Open failed")
		}
	}()
}

func checkHvIsRunning(addr string, retries int) bool {
	url := addr + "/api/ping"
	for i := 0; i < retries; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Get(url) // nolint: gosec
		if err != nil {
			continue
		}
		err = resp.Body.Close()
		if err != nil {
			continue
		}
		if resp.StatusCode < 400 {
			return true
		}
	}
	return false
}

type serversList struct {
	Test      []serversData
	Local     []serversData
	Worldwide []serversData
}

type serversData struct {
	Name            string
	Dmsg            string
	Transport       string
	AddressResolver string
	Routing         string
	UptimeTracker   string
	Launcher        string
}
