// Package visor implements skywire visor.
package visor

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/internal/utclient"
	"github.com/skycoin/skywire/pkg/app/appdisc"
	"github.com/skycoin/skywire/pkg/app/appevent"
	"github.com/skycoin/skywire/pkg/app/appserver"
	"github.com/skycoin/skywire/pkg/app/launcher"
	"github.com/skycoin/skywire/pkg/restart"
	"github.com/skycoin/skywire/pkg/routefinder/rfclient"
	"github.com/skycoin/skywire/pkg/router"
	"github.com/skycoin/skywire/pkg/snet"
	"github.com/skycoin/skywire/pkg/snet/arclient"
	"github.com/skycoin/skywire/pkg/transport"
	"github.com/skycoin/skywire/pkg/util/updater"
	"github.com/skycoin/skywire/pkg/visor/visorconfig"
)

var (
	// ErrAppProcNotRunning represents lookup error for App related calls.
	ErrAppProcNotRunning = errors.New("no process of given app is running")
)

const (
	supportedProtocolVersion = "0.1.0"
	shortHashLen             = 6
	// moduleShutdownTimeout is the timeout given to a module to shutdown cleanly.
	// Otherwise the shutdown logic will continue and report a timeout error.
	moduleShutdownTimeout = time.Second * 2
)

// Visor provides messaging runtime for Apps by setting up all
// necessary connections and performing messaging gateway functions.
type Visor struct {
	reportCh   chan vReport
	closeStack []closeElem

	conf *visorconfig.V1
	log  *logging.Logger

	startedAt     time.Time
	restartCtx    *restart.Context
	updater       *updater.Updater
	uptimeTracker utclient.APIClient

	ebc *appevent.Broadcaster // event broadcaster

	net      *snet.Network
	tpM      *transport.Manager
	arClient arclient.APIClient
	router   router.Router
	rfClient rfclient.Client

	procM       appserver.ProcManager // proc manager
	appL        *launcher.Launcher    // app launcher
	serviceDisc appdisc.Factory
}

type vReport struct {
	src string
	err error
}

type reportFunc func(err error) error

func (v *Visor) makeReporter(src string) reportFunc {
	return func(err error) error {
		v.reportCh <- vReport{src: src, err: err}
		return err
	}
}

func (v *Visor) processReports(log logrus.FieldLogger, ok *bool) {
	if log == nil {
		// nolint:ineffassign
		log = v.log
	}
	for {
		select {
		case report := <-v.reportCh:
			if report.err != nil {
				v.log.WithError(report.err).WithField("_src", report.src).Error()
				if ok != nil {
					*ok = false
				}
			}
		default:
			return
		}
	}
}

type closeElem struct {
	src string
	fn  func() error
}

func (v *Visor) pushCloseStack(src string, fn func() error) {
	v.closeStack = append(v.closeStack, closeElem{src: src, fn: fn})
}

// MasterLogger returns the underlying master logger (currently contained in visor config).
func (v *Visor) MasterLogger() *logging.MasterLogger {
	return v.conf.MasterLogger()
}

// NewVisor constructs new Visor.
func NewVisor(conf *visorconfig.V1, restartCtx *restart.Context) *Visor {
	v := &Visor{
		reportCh:   make(chan vReport, 100),
		log:        conf.MasterLogger().PackageLogger("visor"),
		conf:       conf,
		restartCtx: restartCtx,
	}

	if logLvl, err := logging.LevelFromString(conf.LogLevel); err != nil {
		v.log.WithError(err).Warn("Failed to read log level from config.")
	} else {
		v.conf.MasterLogger().SetLevel(logLvl)
		logging.SetLevel(logLvl)
	}

	v.startedAt = time.Now()

	return v
}

// Start starts visor.
func (v *Visor) Start(ctx context.Context) error {
	log := v.MasterLogger().PackageLogger("visor:startup")
	log.WithField("public_key", v.conf.PK).
		Info("Begin startup.")

	for i, startFn := range initStack() {
		select {
		// quit immediately if startup was interrupted
		case <-ctx.Done():
			return nil
		default:
		}

		name := strings.ToLower(strings.TrimPrefix(filepath.Base(runtime.FuncForPC(reflect.ValueOf(startFn).Pointer()).Name()), "visor.init"))
		start := time.Now()

		log := v.MasterLogger().PackageLogger(fmt.Sprintf("visor:startup:%s", name)).
			WithField("func", fmt.Sprintf("[%d/%d]", i+1, len(initStack())))
		log.Info("Starting module...")

		if err := startFn(ctx, v); err != nil {
			log.WithField("elapsed", time.Since(start)).Error("Failed to start module.")
			v.processReports(log, nil)
			return err
		}

		log.WithField("elapsed", time.Since(start)).Info("Module started successfully.")
	}

	ok := true
	if v.processReports(log, &ok); !ok {
		log.Error("Failed to startup visor.")
		return errors.New("failed to startup visor")
	}

	log.Info("Startup complete!")

	return nil
}

// Network returns skywire network.
func (v *Visor) Network() *snet.Network {
	return v.net
}

// SaveTransport saves transport to `remote`.
func (v *Visor) SaveTransport(ctx context.Context, remote cipher.PubKey, tpType string) (*transport.ManagedTransport, error) {
	return v.tpM.SaveTransport(ctx, remote, tpType)
}

// Close safely stops spawned Apps and Visor.
func (v *Visor) Close() {
	if v == nil {
		return
	}

	log := v.MasterLogger().PackageLogger("visor:shutdown")
	log.Info("Begin shutdown.")

	for i := len(v.closeStack) - 1; i >= 0; i-- {
		ce := v.closeStack[i]

		start := time.Now()
		done := make(chan error, 1)
		t := time.NewTimer(moduleShutdownTimeout)

		log := v.MasterLogger().PackageLogger(fmt.Sprintf("visor:shutdown:%s", ce.src)).
			WithField("func", fmt.Sprintf("[%d/%d]", i+1, len(v.closeStack)))
		log.Info("Shutting down module...")

		go func(ce closeElem) {
			done <- ce.fn()
			close(done)
		}(ce)

		select {
		case err := <-done:
			t.Stop()

			if err != nil {
				log.WithField("elapsed", time.Since(start)).Warn("Module stopped with unexpected result.")
				v.processReports(log, nil)
				continue
			}
			log.WithField("elapsed", time.Since(start)).Info("Module stopped cleanly.")

		case <-t.C:
			log.WithField("elapsed", time.Since(start)).Error("Module timed out.")
		}
	}

	v.processReports(v.log, nil)
	log.Info("Shutdown complete. Goodbye!")
}

// tpDiscClient is a convenience function to obtain transport discovery client.
func (v *Visor) tpDiscClient() transport.DiscoveryClient {
	return v.tpM.Conf.DiscoveryClient
}

// routeFinderClient is a convenience function to obtain route finder client.
func (v *Visor) routeFinderClient() rfclient.Client {
	return v.rfClient
}

// uptimeTrackerClient is a convenience function to obtain uptime tracker client.
func (v *Visor) uptimeTrackerClient() utclient.APIClient {
	return v.uptimeTracker
}

// addressResolverClient is a convenience function to obtain uptime address resovler client.
func (v *Visor) addressResolverClient() arclient.APIClient {
	return v.arClient
}

// unlinkSocketFiles removes unix socketFiles from file system
func unlinkSocketFiles(socketFiles ...string) error {
	for _, f := range socketFiles {
		if err := syscall.Unlink(f); err != nil {
			if !strings.Contains(err.Error(), "no such file or directory") {
				return err
			}
		}
	}

	return nil
}
