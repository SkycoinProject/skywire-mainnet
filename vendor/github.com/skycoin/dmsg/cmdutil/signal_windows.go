// +build windows

package cmdutil

import (
	"os"

	"golang.org/x/sys/windows"
)

func listenSignals() []os.Signal {
	return []os.Signal{windows.SIGINT, windows.SIGTERM, windows.SIGQUIT}
}
