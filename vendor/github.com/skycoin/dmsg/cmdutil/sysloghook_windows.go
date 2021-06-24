// +build windows

package cmdutil

import (
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/skycoin/src/util/logging"
)

func (sf *ServiceFlags) sysLogHook(_ *logging.Logger, _ int) {
}

// LevelFromString returns a logrus.Level and syslog.Priority from a string identifier.
func LevelFromString(s string) (logrus.Level, int, error) {
	switch strings.ToLower(s) {
	case "debug":
		return logrus.DebugLevel, 0, nil
	case "info", "notice":
		return logrus.InfoLevel, 0, nil
	case "warn", "warning":
		return logrus.WarnLevel, 0, nil
	case "error":
		return logrus.ErrorLevel, 0, nil
	case "fatal", "critical":
		return logrus.FatalLevel, 0, nil
	case "panic":
		return logrus.PanicLevel, 0, nil
	default:
		return logrus.DebugLevel, 0, ErrInvalidLogString
	}
}
