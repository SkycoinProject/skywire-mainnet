// +build windows

package syslog

import (
	"errors"

	_ "github.com/konsorten/go-windows-terminal-sequences" // for satisfying logrus dependencies on windows
	"github.com/sirupsen/logrus"
)

// SetupHook sets up syslog hook to the daemon on `addr`.
func SetupHook(_, _ string) (logrus.Hook, error) {
	return nil, errors.New("syslog is not available for this OS")
}
