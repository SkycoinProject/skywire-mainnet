//+build systray

package commands

import (
	"context"
	"sync"

	"github.com/getlantern/systray"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/internal/gui"
)

var (
	stopVisorWg   sync.WaitGroup
	runSysTrayApp bool
)

func init() {
	rootCmd.Flags().StringVar(&tag, "tag", "skywire", "logging tag")
	rootCmd.Flags().StringVar(&syslogAddr, "syslog", "", "syslog server address. E.g. localhost:514")
	rootCmd.Flags().StringVarP(&pprofMode, "pprofmode", "p", "", "pprof profiling mode. Valid values: cpu, mem, mutex, block, trace, http")
	rootCmd.Flags().StringVar(&pprofAddr, "pprofaddr", "localhost:6060", "pprof http port if mode is 'http'")
	rootCmd.Flags().StringVarP(&confPath, "config", "c", "", "config file location. If the value is 'STDIN', config file will be read from stdin.")
	rootCmd.Flags().StringVar(&delay, "delay", "0ns", "start delay (deprecated)") // deprecated
	rootCmd.Flags().BoolVar(&runSysTrayApp, "systray", false, "Run system tray app")
	rootCmd.Flags().BoolVar(&launchBrowser, "launch-browser", false, "open hypervisor web ui (hypervisor only) with system browser")
}

func runApp(args ...string) {
	l := logging.NewMasterLogger()
	sysTrayIcon, err := gui.ReadSysTrayIcon()
	if err != nil {
		l.WithError(err).Fatalln("Failed to read system tray icon")
	}

	go func() {
		runVisor(args)
		gui.Stop()
	}()

	conf := initConfig(l, args, confPath)

	systray.Run(gui.GetOnGUIReady(sysTrayIcon, conf), gui.OnGUIQuit)

}

func stopSystray(cancel context.CancelFunc) {
	stopVisorWg.Add(1)
	defer stopVisorWg.Done()

	gui.SetStopVisorFn(func() {
		cancel()
		stopVisorWg.Wait()
	})
}

func quitSystray() {
	systray.Quit()
}
