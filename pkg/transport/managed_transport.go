package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/httputil"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/internal/netutil"
	"github.com/skycoin/skywire/pkg/app/appevent"
	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/skyenv"
	"github.com/skycoin/skywire/pkg/transport/network"
)

const logWriteInterval = time.Second * 3

// Records number of managedTransports.
var mTpCount int32

var (
	// ErrNotServing is the error returned when a transport is no longer served.
	ErrNotServing = errors.New("transport is no longer being served")

	// ErrConnAlreadyExists occurs when an underlying transport connection already exists.
	ErrConnAlreadyExists = errors.New("underlying transport connection already exists")
)

// ManagedTransportConfig is a configuration for managed transport.
type ManagedTransportConfig struct {
	client         network.Client
	ebc            *appevent.Broadcaster
	DC             DiscoveryClient
	LS             LogStore
	RemotePK       cipher.PubKey
	TransportLabel Label
}

// ManagedTransport manages a direct line of communication between two visor nodes.
// There is a single underlying connection between two edges.
// Initial dialing can be requested by either edge of the connection.
type ManagedTransport struct {
	log *logging.Logger

	rPK        cipher.PubKey
	Entry      Entry
	LogEntry   *LogEntry
	logUpdates uint32

	dc DiscoveryClient
	ls LogStore

	client network.Client
	conn   network.Conn
	connCh chan struct{}
	connMx sync.Mutex

	done chan struct{}
	once sync.Once
	wg   sync.WaitGroup
}

// NewManagedTransport creates a new ManagedTransport.
func NewManagedTransport(conf ManagedTransportConfig) *ManagedTransport {
	aPK, bPK := conf.client.PK(), conf.RemotePK
	mt := &ManagedTransport{
		log:      logging.MustGetLogger(fmt.Sprintf("tp:%s", conf.RemotePK.String()[:6])),
		rPK:      conf.RemotePK,
		dc:       conf.DC,
		ls:       conf.LS,
		client:   conf.client,
		Entry:    MakeEntry(aPK, bPK, conf.client.Type(), true, conf.TransportLabel),
		LogEntry: new(LogEntry),
		connCh:   make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	mt.wg.Add(2)
	return mt
}

// Serve serves and manages the transport.
func (mt *ManagedTransport) Serve(readCh chan<- routing.Packet) {
	defer mt.wg.Done()

	log := mt.log.
		WithField("tp_id", mt.Entry.ID).
		WithField("remote_pk", mt.rPK).
		WithField("tp_index", atomic.AddInt32(&mTpCount, 1))

	log.Info("Serving.")

	defer func() {
		mt.close()
		log.WithField("remaining_tps", atomic.AddInt32(&mTpCount, -1)).
			Info("Stopped serving.")
	}()

	go mt.readLoop(readCh)
	mt.logLoop()
}

// readLoop continuously reads packets from the underlying transport connection
// and sends them to readCh
// This is a blocking call
func (mt *ManagedTransport) readLoop(readCh chan<- routing.Packet) {
	log := mt.log.WithField("src", "read_loop")
	defer func() {
		mt.wg.Done()
		log.Debug("Closed read loop.")
	}()
	for {
		p, err := mt.readPacket()
		if err != nil {
			log.WithError(err).Warn("Failed to read packet, closing transport")
			mt.close()
			return
		}
		select {
		case <-mt.done:
			return
		case readCh <- p:
		}
	}
}

// logLoop continuously stores transport data in the log entry,
// in case there is data to store
// This is a blocking call
func (mt *ManagedTransport) logLoop() {
	// Ensure logs tp logs are up to date before closing.
	defer mt.recordLog()
	logTicker := time.NewTicker(logWriteInterval)
	for {
		select {
		case <-mt.done:
			logTicker.Stop()
			return
		case <-logTicker.C:
			mt.recordLog()
		}
	}
}

func (mt *ManagedTransport) isServing() bool {
	select {
	case <-mt.done:
		return false
	default:
		return true
	}
}

// Close implements io.Closer
// It also waits for transport to stop serving before it returns.
// It only returns an error if transport status update fails.
func (mt *ManagedTransport) Close() (err error) {
	mt.close()
	mt.wg.Wait()
	return nil
}

// IsClosed returns true when the transport is closed
// This instance cannot be used anymore and should be discarded
func (mt *ManagedTransport) IsClosed() bool {
	select {
	case <-mt.done:
		return true
	default:
		return false
	}
}

// close underlying connection and update entry status in transport discovery
// todo: this currently performs http request to discovery service
// it only makes sense to wait for the completion if we are closing the visor itself,
// regular transport close operations should probably call it concurrently
// need to find a way to handle this properly (done channel in return?)
func (mt *ManagedTransport) close() {
	select {
	case <-mt.done:
		return
	default:
		close(mt.done)
	}
	mt.connMx.Lock()
	close(mt.connCh)
	if mt.conn != nil {
		if err := mt.conn.Close(); err != nil {
			log.WithError(err).Warn("Failed to close underlying connection.")
		}
		mt.conn = nil
	}
	mt.connMx.Unlock()
	_ = mt.deleteFromDiscovery() //nolint:errcheck
}

// Accept accepts a new underlying connection.
func (mt *ManagedTransport) Accept(ctx context.Context, conn network.Conn) error {
	mt.connMx.Lock()
	defer mt.connMx.Unlock()

	if conn.Network() != mt.Type() {
		return ErrWrongNetwork
	}

	if !mt.isServing() {
		mt.log.WithError(ErrNotServing).Debug()
		if err := conn.Close(); err != nil {
			mt.log.WithError(err).
				Warn("Failed to close newly accepted connection.")
		}
		return ErrNotServing
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()

	mt.log.Debug("Performing settlement handshake...")
	if err := MakeSettlementHS(false).Do(ctx, mt.dc, conn, mt.client.SK()); err != nil {
		return fmt.Errorf("settlement handshake failed: %w", err)
	}

	mt.log.Debug("Setting underlying connection...")
	return mt.setConn(conn)
}

// Dial dials a new underlying connection.
func (mt *ManagedTransport) Dial(ctx context.Context) error {
	mt.connMx.Lock()
	defer mt.connMx.Unlock()

	if !mt.isServing() {
		return ErrNotServing
	}

	if mt.conn != nil {
		return nil
	}
	return mt.dial(ctx)
}

func (mt *ManagedTransport) dial(ctx context.Context) error {
	conn, err := mt.client.Dial(ctx, mt.rPK, skyenv.DmsgTransportPort)
	if err != nil {
		return fmt.Errorf("snet.Dial: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()

	if err := MakeSettlementHS(true).Do(ctx, mt.dc, conn, mt.client.SK()); err != nil {
		return fmt.Errorf("settlement handshake failed: %w", err)
	}

	if err := mt.setConn(conn); err != nil {
		return fmt.Errorf("setConn: %w", err)
	}

	return nil
}

func (mt *ManagedTransport) isLeastSignificantEdge() bool {
	sorted := SortEdges(mt.Entry.Edges[0], mt.Entry.Edges[1])
	return sorted[0] == mt.client.PK()
}

/*
	<<< UNDERLYING CONNECTION >>>
*/

func (mt *ManagedTransport) getConn() network.Conn {
	if !mt.isServing() {
		return nil
	}

	mt.connMx.Lock()
	conn := mt.conn
	mt.connMx.Unlock()
	return conn
}

// setConn sets 'mt.conn' (the underlying connection).
// If 'mt.conn' is already occupied, close the newly introduced connection.
func (mt *ManagedTransport) setConn(newConn network.Conn) error {

	if mt.conn != nil {
		if mt.isLeastSignificantEdge() {
			mt.log.Debug("Underlying conn already exists, closing new conn.")
			if err := newConn.Close(); err != nil {
				log.WithError(err).Warn("Failed to close new conn.")
			}
			return ErrConnAlreadyExists
		}

		mt.log.Debug("Underlying conn already exists, closing old conn.")
		if err := mt.conn.Close(); err != nil {
			log.WithError(err).Warn("Failed to close old conn.")
		}
		mt.conn = nil
	}

	// Set new underlying connection.
	mt.conn = newConn
	select {
	case mt.connCh <- struct{}{}:
		mt.log.Debug("Sent signal to 'mt.connCh'.")
	default:
	}

	return nil
}

func (mt *ManagedTransport) deleteFromDiscovery() error {
	retrier := netutil.NewRetrier(1*time.Second, 5, 2)
	return retrier.Do(func() error {
		err := mt.dc.DeleteTransport(context.Background(), mt.Entry.ID)
		if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
			mt.log.
				WithError(err).
				WithField("temporary", true).
				Warn("Failed to update transport status.")
			return err
		}
		if httpErr, ok := err.(*httputil.HTTPError); ok && httpErr.Status == http.StatusNotFound {
			return nil
		}
		return err
	})
}

/*
	<<< PACKET MANAGEMENT >>>
*/

// WritePacket writes a packet to the remote.
func (mt *ManagedTransport) WritePacket(ctx context.Context, packet routing.Packet) error {
	mt.connMx.Lock()
	defer mt.connMx.Unlock()

	if mt.conn == nil {
		return fmt.Errorf("write packet: cannot write to conn, conn is not set up")
	}

	n, err := mt.conn.Write(packet)
	if err != nil {
		mt.close()
		return err
	}
	if n > routing.PacketHeaderSize {
		mt.logSent(uint64(n - routing.PacketHeaderSize))
	}
	return nil
}

// WARNING: Not thread safe.
func (mt *ManagedTransport) readPacket() (packet routing.Packet, err error) {
	log := mt.log.WithField("func", "readPacket")

	var conn network.Conn
	for {
		if conn = mt.getConn(); conn != nil {
			break
		}
		select {
		case <-mt.done:
			return nil, ErrNotServing
		case <-mt.connCh:
		}
	}

	log.Debug("Awaiting packet...")

	h := make(routing.Packet, routing.PacketHeaderSize)
	if _, err = io.ReadFull(conn, h); err != nil {
		log.WithError(err).Debugf("Failed to read packet header.")
		return nil, err
	}
	log.WithField("header_len", len(h)).WithField("header_raw", h).Debug("Read packet header.")

	p := make([]byte, h.Size())
	if _, err = io.ReadFull(conn, p); err != nil {
		log.WithError(err).Debugf("Failed to read packet payload.")
		return nil, err
	}
	log.WithField("payload_len", len(p)).Debug("Read packet payload.")

	packet = append(h, p...)
	if n := len(packet); n > routing.PacketHeaderSize {
		mt.logRecv(uint64(n - routing.PacketHeaderSize))
	}

	log.WithField("type", packet.Type().String()).
		WithField("rt_id", packet.RouteID()).
		WithField("size", packet.Size()).
		Debug("Received packet.")
	return packet, nil
}

/*
	<<< TRANSPORT LOGGING >>>
*/

func (mt *ManagedTransport) logSent(b uint64) {
	mt.LogEntry.AddSent(b)
	atomic.AddUint32(&mt.logUpdates, 1)
}

func (mt *ManagedTransport) logRecv(b uint64) {
	mt.LogEntry.AddRecv(b)
	atomic.AddUint32(&mt.logUpdates, 1)
}

// logMod flushes the number of operations performed in this transport
// and returns true if it was bigger than 0
func (mt *ManagedTransport) logMod() bool {
	if ops := atomic.SwapUint32(&mt.logUpdates, 0); ops > 0 {
		mt.log.Infof("entry log: recording %d operations", ops)
		return true
	}
	return false
}

// records this transport's log, in case there is data to be logged
func (mt *ManagedTransport) recordLog() {
	if !mt.logMod() {
		return
	}
	if err := mt.ls.Record(mt.Entry.ID, mt.LogEntry); err != nil {
		log.WithError(err).Warn("Failed to record log entry.")
	}
}

// Remote returns the remote public key.
func (mt *ManagedTransport) Remote() cipher.PubKey { return mt.rPK }

// Type returns the transport type.
func (mt *ManagedTransport) Type() network.Type { return mt.client.Type() }
