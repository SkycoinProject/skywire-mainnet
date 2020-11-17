//+build windows

package visor

import "context"

// InitDmsgpty initializes dmsgpty.
func InitDmsgpty(_ context.Context, v *Visor) error {
	report := v.makeReporter("dmsgpty")
	conf := v.conf.Dmsgpty

	if conf == nil {
		v.log.Info("'dmsgpty' is not configured, skipping.")
		return report(nil)
	}

	v.log.Error("dmsgpty is not supported on windows.")
	return report(nil)
}
