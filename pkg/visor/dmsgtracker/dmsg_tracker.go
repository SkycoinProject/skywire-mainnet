package dmsgtracker

import (
	"context"
	"io"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/dmsgctrl"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/pkg/skyenv"
)

// Default values for DmsgTrackerManager
const (
	DefaultDTMUpdateInterval = time.Second * 30
	DefaultDTMUpdateTimeout  = time.Second * 10
)

// DmsgClientSummary summaries a dmsg client.
type DmsgClientSummary struct {
	PK        cipher.PubKey `json:"public_key"`
	ServerPK  cipher.PubKey `json:"server_public_key"`
	RoundTrip time.Duration `json:"round_trip"`
}

// DmsgTracker tracks a dmsg client.
type DmsgTracker struct {
	sum  DmsgClientSummary // dmsg summary
	ctrl *dmsgctrl.Control // dmsg ctrl
}

// NewDmsgTracker creates a new DmsgTracker.
func NewDmsgTracker(ctx context.Context, dmsgC *dmsg.Client, pk cipher.PubKey) (dt *DmsgTracker, err error) {
	conn, err := dmsgC.DialStream(ctx, dmsg.Addr{PK: pk, Port: skyenv.DmsgCtrlPort})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = conn.Close() //nolint:errcheck
		}
	}()

	ctrl := dmsgctrl.ControlStream(conn)

	dur, err := ctrl.Ping(ctx)
	if err != nil {
		return nil, err
	}

	dt = &DmsgTracker{
		sum: DmsgClientSummary{
			PK:        conn.RawRemoteAddr().PK,
			ServerPK:  conn.ServerPK(),
			RoundTrip: dur,
		},
		ctrl: ctrl,
	}

	return dt, nil
}

// Update updates the dmsg client summary.
func (dt *DmsgTracker) Update(ctx context.Context) error {
	dur, err := dt.ctrl.Ping(ctx)
	if err != nil {
		return err
	}

	dt.sum.RoundTrip = dur
	return nil
}

// Manager tracks round trip durations for dmsg client connections.
type Manager struct {
	updateInterval time.Duration
	updateTimeout  time.Duration

	log logrus.FieldLogger
	dc  *dmsg.Client
	dm  map[cipher.PubKey]*DmsgTracker
	mx  sync.Mutex

	done     chan struct{}
	doneOnce sync.Once
}

// NewDmsgTrackerManager creates a new dmsg tracker manager.
func NewDmsgTrackerManager(log logrus.FieldLogger, dc *dmsg.Client, updateInterval, updateTimeout time.Duration) *Manager {
	if log == nil {
		log = logging.MustGetLogger("dmsg_trackers")
	}
	if updateInterval == 0 {
		updateInterval = DefaultDTMUpdateInterval
	}
	if updateTimeout == 0 {
		updateTimeout = DefaultDTMUpdateTimeout
	}

	dtm := &Manager{
		updateInterval: updateInterval,
		updateTimeout:  updateTimeout,
		log:            log,
		dc:             dc,
		dm:             make(map[cipher.PubKey]*DmsgTracker),
		done:           make(chan struct{}),
	}

	if dc != nil {
		go dtm.serve()
	}

	return dtm
}

// Serve serves the dmsg tracker manager.
func (dtm *Manager) serve() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-dtm.done
		cancel()
	}()

	t := time.NewTicker(dtm.updateInterval)
	defer t.Stop()

	for {
		select {
		case <-dtm.done:
			return
		case <-t.C:
			ctx, cancel := context.WithDeadline(ctx, time.Now().Add(dtm.updateTimeout))
			dtm.mx.Lock()
			dtm.updateAllTrackers(ctx, dtm.dm)
			dtm.mx.Unlock()
			cancel()
		}
	}
}

func (dtm *Manager) updateAllTrackers(ctx context.Context, dts map[cipher.PubKey]*DmsgTracker) {
	log := dtm.log.WithField("func", funcName())

	type errReport struct {
		pk  cipher.PubKey
		err error
	}

	dtsLen := len(dts)
	errCh := make(chan errReport, dtsLen)
	defer close(errCh)

	for _, te := range dts {
		te := te

		go func() {
			err := te.Update(ctx)
			errCh <- errReport{pk: te.sum.PK, err: err}
		}()
	}

	for i := 0; i < dtsLen; i++ {
		if r := <-errCh; r.err != nil {
			log.WithError(r.err).
				WithField("client_pk", r.pk).
				Warn("Removing dmsg client tracker.")
			delete(dts, r.pk)
		}
	}
}

// MustGet obtains a DmsgClientSummary of the client of given pk.
// If one is not found internally, a new tracker stream is to be established, returning error on failure.
func (dtm *Manager) MustGet(ctx context.Context, pk cipher.PubKey) (DmsgClientSummary, error) {
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(dtm.updateTimeout))
	defer cancel()

	dtm.mx.Lock()
	defer dtm.mx.Unlock()

	if isDone(dtm.done) {
		return DmsgClientSummary{}, io.ErrClosedPipe
	}

	if e, ok := dtm.dm[pk]; ok && !isDone(e.ctrl.Done()) {
		return e.sum, nil
	}

	dt, err := NewDmsgTracker(ctx, dtm.dc, pk)
	if err != nil {
		return DmsgClientSummary{}, err
	}

	dtm.dm[pk] = dt
	return dt.sum, nil
}

// Get obtains a DmsgClientSummary of the client with given public key.
func (dtm *Manager) Get(pk cipher.PubKey) (DmsgClientSummary, bool) {
	dtm.mx.Lock()
	defer dtm.mx.Unlock()

	if isDone(dtm.done) {
		return DmsgClientSummary{}, false
	}

	return dtm.get(pk)
}

// GetBulk obtains bulk dmsg client summaries.
func (dtm *Manager) GetBulk(pks []cipher.PubKey) []DmsgClientSummary {
	dtm.mx.Lock()
	defer dtm.mx.Unlock()

	out := make([]DmsgClientSummary, 0, len(pks))

	for _, pk := range pks {
		dt, ok := dtm.dm[pk]
		if !ok {
			continue
		}
		out = append(out, dt.sum)
	}

	sort.Slice(out, func(i, j int) bool {
		outI := out[i].PK.Big()
		outJ := out[j].PK.Big()
		return outI.Cmp(outJ) < 0
	})

	return out
}

func (dtm *Manager) get(pk cipher.PubKey) (DmsgClientSummary, bool) {
	dt, ok := dtm.dm[pk]
	if !ok {
		return DmsgClientSummary{}, false
	}

	return dt.sum, true
}

// Close implements io.Closer
func (dtm *Manager) Close() error {
	log := dtm.log.WithField("func", funcName())

	dtm.mx.Lock()
	defer dtm.mx.Unlock()

	closed := false

	dtm.doneOnce.Do(func() {
		closed = true
		close(dtm.done)

		for pk, dt := range dtm.dm {
			if err := dt.ctrl.Close(); err != nil {
				log.WithError(err).
					WithField("client_pk", pk).
					Warn("Dmsg client closed with error.")
			}
		}
	})

	if !closed {
		return io.ErrClosedPipe
	}

	return nil
}

func isDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func funcName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}
