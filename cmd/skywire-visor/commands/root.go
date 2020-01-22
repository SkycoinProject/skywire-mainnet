package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"log/syslog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/pkg/profile"
	logrussyslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/spf13/cobra"

	"github.com/SkycoinProject/skywire-mainnet/internal/utclient"
	"github.com/SkycoinProject/skywire-mainnet/pkg/restart"
	"github.com/SkycoinProject/skywire-mainnet/pkg/util/pathutil"
	"github.com/SkycoinProject/skywire-mainnet/pkg/visor"
)

// TODO(evanlinjin): Determine if this is still needed.
//import _ "net/http/pprof" // used for HTTP profiling

const configEnv = "SW_CONFIG"
const defaultShutdownTimeout = visor.Duration(10 * time.Second)

type runCfg struct {
	syslogAddr   string
	tag          string
	cfgFromStdin bool
	profileMode  string
	port         string
	startDelay   string
	args         []string
	configPath   *string

	profileStop  func()
	logger       *logging.Logger
	masterLogger *logging.MasterLogger
	conf         visor.Config
	node         *visor.Node
	restartCtx   *restart.Context
}

var cfg *runCfg

var rootCmd = &cobra.Command{
	Use:   "skywire-visor [config-path]",
	Short: "Visor for skywire",
	Run: func(_ *cobra.Command, args []string) {
		cfg.args = args

		cfg.startProfiler().
			startLogger().
			readConfig().
			runNode().
			waitOsSignals().
			stopNode()
	},
	Version: visor.Version,
}

func init() {
	cfg = &runCfg{}
	rootCmd.Flags().StringVarP(&cfg.syslogAddr, "syslog", "", "none", "syslog server address. E.g. localhost:514")
	rootCmd.Flags().StringVarP(&cfg.tag, "tag", "", "skywire", "logging tag")
	rootCmd.Flags().BoolVarP(&cfg.cfgFromStdin, "stdin", "i", false, "read config from STDIN")
	rootCmd.Flags().StringVarP(&cfg.profileMode, "profile", "p", "none", "enable profiling with pprof. Mode:  none or one of: [cpu, mem, mutex, block, trace, http]")
	rootCmd.Flags().StringVarP(&cfg.port, "port", "", "6060", "port for http-mode of pprof")
	rootCmd.Flags().StringVarP(&cfg.startDelay, "delay", "", "0ns", "delay before visor start")

	cfg.restartCtx = restart.CaptureContext()
}

// Execute executes root CLI command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func (cfg *runCfg) startProfiler() *runCfg {
	var option func(*profile.Profile)

	switch cfg.profileMode {
	case "none":
		cfg.profileStop = func() {}
		return cfg
	case "http":
		go func() {
			log.Println(http.ListenAndServe(fmt.Sprintf("localhost:%v", cfg.port), nil))
		}()

		cfg.profileStop = func() {}

		return cfg
	case "cpu":
		option = profile.CPUProfile
	case "mem":
		option = profile.MemProfile
	case "mutex":
		option = profile.MutexProfile
	case "block":
		option = profile.BlockProfile
	case "trace":
		option = profile.TraceProfile
	}

	cfg.profileStop = profile.Start(profile.ProfilePath("./logs/"+cfg.tag), option).Stop

	return cfg
}

func (cfg *runCfg) startLogger() *runCfg {
	cfg.masterLogger = logging.NewMasterLogger()
	cfg.logger = cfg.masterLogger.PackageLogger(cfg.tag)

	if cfg.syslogAddr != "none" {
		hook, err := logrussyslog.NewSyslogHook("udp", cfg.syslogAddr, syslog.LOG_INFO, cfg.tag)
		if err != nil {
			cfg.logger.Error("Unable to connect to syslog daemon:", err)
		} else {
			cfg.masterLogger.AddHook(hook)
			cfg.masterLogger.Out = ioutil.Discard
		}
	}

	return cfg
}

func (cfg *runCfg) readConfig() *runCfg {
	var rdr io.Reader

	if !cfg.cfgFromStdin {
		configPath := pathutil.FindConfigPath(cfg.args, 0, configEnv, pathutil.NodeDefaults())

		file, err := os.Open(filepath.Clean(configPath))
		if err != nil {
			cfg.logger.Fatalf("Failed to open config: %s", err)
		}

		defer func() {
			if err := file.Close(); err != nil {
				cfg.logger.Warnf("Failed to close config file: %v", err)
			}
		}()

		cfg.logger.Info("Reading config from %v", configPath)

		rdr = file
		cfg.configPath = &configPath
	} else {
		cfg.logger.Info("Reading config from STDIN")
		rdr = bufio.NewReader(os.Stdin)
	}

	cfg.conf = visor.Config{}
	if err := json.NewDecoder(rdr).Decode(&cfg.conf); err != nil {
		cfg.logger.Fatalf("Failed to decode %s: %s", rdr, err)
	}

	fmt.Println("TCP Factory conf:", cfg.conf.STCP)

	return cfg
}

func (cfg *runCfg) runNode() *runCfg {
	startDelay, err := time.ParseDuration(cfg.startDelay)
	if err != nil {
		cfg.logger.Warnf("Using no visor start delay due to parsing failure: %v", err)

		startDelay = time.Duration(0)
	}

	if startDelay != 0 {
		cfg.logger.Infof("Visor start delay is %v, waiting...", startDelay)
	}

	time.Sleep(startDelay)

	if cfg.conf.DmsgPty != nil {
		err = visor.UnlinkSocketFiles(cfg.conf.AppServerSockFile, cfg.conf.DmsgPty.CLIAddr)
	} else {
		err = visor.UnlinkSocketFiles(cfg.conf.AppServerSockFile)
	}

	if err != nil {
		cfg.logger.Fatal("failed to unlink socket files: ", err)
	}

	node, err := visor.NewNode(&cfg.conf, cfg.masterLogger, cfg.restartCtx, cfg.configPath)
	if err != nil {
		cfg.logger.Fatal("Failed to initialize node: ", err)
	}

	if cfg.conf.Uptime.Tracker != "" {
		uptimeTracker, err := utclient.NewHTTP(cfg.conf.Uptime.Tracker, cfg.conf.Node.StaticPubKey, cfg.conf.Node.StaticSecKey)
		if err != nil {
			cfg.logger.Error("Failed to connect to uptime tracker: ", err)
		} else {
			ticker := time.NewTicker(1 * time.Second)

			go func() {
				for range ticker.C {
					ctx := context.Background()
					if err := uptimeTracker.UpdateNodeUptime(ctx); err != nil {
						cfg.logger.Error("Failed to update node uptime: ", err)
					}
				}
			}()
		}
	}

	go func() {
		if err := node.Start(); err != nil {
			cfg.logger.Fatal("Failed to start node: ", err)
		}
	}()

	if cfg.conf.ShutdownTimeout == 0 {
		cfg.conf.ShutdownTimeout = defaultShutdownTimeout
	}

	cfg.node = node

	return cfg
}

func (cfg *runCfg) stopNode() *runCfg {
	defer cfg.profileStop()

	if err := cfg.node.Close(); err != nil {
		if !strings.Contains(err.Error(), "closed") {
			cfg.logger.Fatal("Failed to close node: ", err)
		}
	}

	return cfg
}

func (cfg *runCfg) waitOsSignals() *runCfg {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}...)
	<-ch

	go func() {
		select {
		case <-time.After(time.Duration(cfg.conf.ShutdownTimeout)):
			cfg.logger.Fatal("Timeout reached: terminating")
		case s := <-ch:
			cfg.logger.Fatalf("Received signal %s: terminating", s)
		}
	}()

	return cfg
}
