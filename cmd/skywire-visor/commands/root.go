package commands

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof" // nolint:gosec // https://golang.org/doc/diagnostics.html#profiling
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/profile"
	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cmdutil"
	"github.com/skycoin/dmsg/discord"
	"github.com/skycoin/skycoin/src/util/logging"
	"github.com/spf13/cobra"

	"github.com/skycoin/skywire/pkg/restart"
	"github.com/skycoin/skywire/pkg/syslog"
	"github.com/skycoin/skywire/pkg/visor"
	"github.com/skycoin/skywire/pkg/visor/visorconfig"
)

var restartCtx = restart.CaptureContext()

const (
	defaultConfigName = "skywire-config.json"
)

var (
	tag        string
	syslogAddr string
	pprofMode  string
	pprofAddr  string
	confPath   string
	delay      string
)

func init() {
	rootCmd.Flags().StringVar(&tag, "tag", "skywire", "logging tag")
	rootCmd.Flags().StringVar(&syslogAddr, "syslog", "", "syslog server address. E.g. localhost:514")
	rootCmd.Flags().StringVarP(&pprofMode, "pprofmode", "p", "", "pprof profiling mode. Valid values: cpu, mem, mutex, block, trace, http")
	rootCmd.Flags().StringVar(&pprofAddr, "pprofaddr", "localhost:6060", "pprof http port if mode is 'http'")
	rootCmd.Flags().StringVarP(&confPath, "config", "c", "", "config file location. If the value is 'STDIN', config file will be read from stdin.")
	rootCmd.Flags().StringVar(&delay, "delay", "0ns", "start delay (deprecated)") // deprecated
}

var rootCmd = &cobra.Command{
	Use:   "skywire-visor",
	Short: "Skywire visor",
	Run: func(_ *cobra.Command, args []string) {
		log := initLogger(tag, syslogAddr)

		if discordWebhookURL := discord.GetWebhookURLFromEnv(); discordWebhookURL != "" {
			// Workaround for Discord logger hook. Actually, it's Info.
			log.Error(discord.StartLogMessage)
			defer log.Error(discord.StopLogMessage)
		} else {
			log.Info(discord.StartLogMessage)
			defer log.Info(discord.StopLogMessage)
		}

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

			// TODO(nkryuchkov): decide if it's needed, prevents Windows build
			// Detach child from parent. TODO: This may be unnecessary.
			/*if _, err := syscall.Setsid(); err != nil {
				log.WithError(err).Errorf("Failed to call setsid()")
			}*/
		}

		time.Sleep(delayDuration)

		if _, err := buildinfo.Get().WriteTo(log.Out); err != nil {
			log.WithError(err).Error("Failed to output build info.")
		}

		stopPProf := initPProf(log, tag, pprofMode, pprofAddr)
		defer stopPProf()

		conf := initConfig(log, args, confPath)

		v := visor.NewVisor(conf, restartCtx)

		if err := v.Start(context.Background()); err != nil {
			log.Fatal("Failed to start visor.")
		}

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()

		// Wait.
		<-ctx.Done()

		v.Close()
	},
	Version: buildinfo.Version(),
}

// Execute executes root CLI command.
func Execute() {
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

	if discordWebhookURL := discord.GetWebhookURLFromEnv(); discordWebhookURL != "" {
		discordOpts := discord.GetDefaultOpts()
		hook := discord.NewHook(tag, discordWebhookURL, discordOpts...)
		log.AddHook(hook)
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

	return conf
}
