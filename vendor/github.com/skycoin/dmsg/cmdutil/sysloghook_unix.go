// +build !windows

package cmdutil

import (
	"log/syslog"
	"strings"

	"github.com/sirupsen/logrus"
	logrussyslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/skycoin/skycoin/src/util/logging"
)

func (sf *ServiceFlags) sysLogHook(log *logging.Logger, sysLvl int) {
	hook, err := logrussyslog.NewSyslogHook(sf.SyslogNet, sf.Syslog, syslog.Priority(sysLvl), sf.Tag)
	if err != nil {
		log.WithError(err).
			WithField("net", sf.SyslogNet).
			WithField("addr", sf.Syslog).
			Fatal("Failed to connect to syslog daemon.")
	}
	logging.AddHook(hook)
}

// LevelFromString returns a logrus.Level and syslog.Priority from a string identifier.
func LevelFromString(s string) (logrus.Level, int, error) {
	switch strings.ToLower(s) {
	case "debug":
		return logrus.DebugLevel, int(syslog.LOG_DEBUG), nil
	case "info", "notice":
		return logrus.InfoLevel, int(syslog.LOG_INFO), nil
	case "warn", "warning":
		return logrus.WarnLevel, int(syslog.LOG_WARNING), nil
	case "error":
		return logrus.ErrorLevel, int(syslog.LOG_ERR), nil
	case "fatal", "critical":
		return logrus.FatalLevel, int(syslog.LOG_CRIT), nil
	case "panic":
		return logrus.PanicLevel, int(syslog.LOG_EMERG), nil
	default:
		return logrus.DebugLevel, int(syslog.LOG_DEBUG), ErrInvalidLogString
	}
}
