//+build !systray

package commands

import (
	"context"
)

func init() {
	rootCmd.Flags().StringVar(&tag, "tag", "skywire", "logging tag")
	rootCmd.Flags().StringVar(&syslogAddr, "syslog", "", "syslog server address. E.g. localhost:514")
	rootCmd.Flags().StringVarP(&pprofMode, "pprofmode", "p", "", "pprof profiling mode. Valid values: cpu, mem, mutex, block, trace, http")
	rootCmd.Flags().StringVar(&pprofAddr, "pprofaddr", "localhost:6060", "pprof http port if mode is 'http'")
	rootCmd.Flags().StringVarP(&confPath, "config", "c", "", "config file location. If the value is 'STDIN', config file will be read from stdin.")
	rootCmd.Flags().StringVar(&delay, "delay", "0ns", "start delay (deprecated)") // deprecated
	rootCmd.Flags().BoolVar(&launchBrowser, "launch-browser", false, "open hypervisor web ui (hypervisor only) with system browser")
}

func runApp(args ...string) {
	runVisor(args)
}

// stopSystray is a stub
func stopSystray(_ context.CancelFunc) {
}

// quitSystray is a stub
func quitSystray() {
}
