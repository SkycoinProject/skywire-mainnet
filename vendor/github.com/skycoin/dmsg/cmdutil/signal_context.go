package cmdutil

import (
	"context"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"
)

// SignalContext returns a context that cancels on given syscall signals.
func SignalContext(ctx context.Context, log logrus.FieldLogger) (context.Context, context.CancelFunc) {
	if log == nil {
		log = logrus.New()
	}

	ctx, cancel := context.WithCancel(ctx)

	ch := make(chan os.Signal)
	listenSigs := listenSignals()
	signal.Notify(ch, listenSigs...)

	go func() {
		select {
		case sig := <-ch:
			log.WithField("signal", sig).
				Info("Closing with received signal.")
		case <-ctx.Done():
		}
		cancel()
	}()

	return ctx, cancel
}
