package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/pkg/app/appevent"
	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/skyenv"
	"github.com/skycoin/skywire/pkg/transport/network"
	"github.com/skycoin/skywire/pkg/transport/network/addrresolver"
)

// ManagerConfig configures a Manager.
type ManagerConfig struct {
	PubKey          cipher.PubKey
	SecKey          cipher.SecKey
	DiscoveryClient DiscoveryClient
	LogStore        LogStore
}

// Manager manages Transports.
type Manager struct {
	Logger   *logging.Logger
	Conf     *ManagerConfig
	tps      map[uuid.UUID]*ManagedTransport
	arClient addrresolver.APIClient
	ebc      *appevent.Broadcaster

	readCh    chan routing.Packet
	mx        sync.RWMutex
	wgMu      sync.Mutex
	wg        sync.WaitGroup
	serveOnce sync.Once // ensure we only serve once.
	closeOnce sync.Once // ensure we only close once.
	done      chan struct{}

	factory    network.ClientFactory
	netClients map[network.Type]network.Client
}

// NewManager creates a Manager with the provided configuration and transport factories.
// 'factories' should be ordered by preference.
func NewManager(log *logging.Logger, arClient addrresolver.APIClient, ebc *appevent.Broadcaster, config *ManagerConfig, factory network.ClientFactory) (*Manager, error) {
	if log == nil {
		log = logging.MustGetLogger("tp_manager")
	}
	tm := &Manager{
		Logger:     log,
		Conf:       config,
		tps:        make(map[uuid.UUID]*ManagedTransport),
		readCh:     make(chan routing.Packet, 20),
		done:       make(chan struct{}),
		netClients: make(map[network.Type]network.Client),
		arClient:   arClient,
		factory:    factory,
		ebc:        ebc,
	}
	return tm, nil
}

// Serve runs listening loop across all registered factories.
func (tm *Manager) Serve(ctx context.Context) {
	tm.serveOnce.Do(func() {
		tm.serve(ctx)
	})
}

func (tm *Manager) serve(ctx context.Context) {
	tm.initClients()
	tm.runClients(ctx)
	go tm.cleanupTransports(ctx)
	// todo: add "persistent transports" loop that will continuously try
	// to establish transports from that list (unless they are already running)
	// persistent transports should come from visor configuration and will
	// allow user to set connections to other visors that:
	// 1. will be established upon visor startup
	// 2. will be redialed when broken
	tm.Logger.Info("transport manager is serving.")
}

func (tm *Manager) initClients() {
	acceptedNetworks := []network.Type{network.STCP, network.STCPR, network.SUDPH, network.DMSG}
	for _, netType := range acceptedNetworks {
		client, err := tm.factory.MakeClient(netType)
		if err != nil {
			tm.Logger.Warnf("Cannot initialize %s transport client", netType)
			continue
		}
		tm.netClients[netType] = client
	}
}

func (tm *Manager) runClients(ctx context.Context) {
	if tm.isClosing() {
		return
	}
	for _, client := range tm.netClients {
		tm.Logger.Infof("Serving %s network", client.Type())
		err := client.Start()
		if err != nil {
			tm.Logger.WithError(err).Errorf("Failed to listen on %s network", client.Type())
			continue
		}
		lis, err := client.Listen(skyenv.DmsgTransportPort)
		if err != nil {
			tm.Logger.WithError(err).Fatalf("failed to listen on network '%s' of port '%d'",
				client.Type(), skyenv.DmsgTransportPort)
			return
		}
		tm.Logger.Infof("listening on network: %s", client.Type())
		tm.wgMu.Lock()
		tm.wg.Add(1)
		tm.wgMu.Unlock()
		go tm.acceptTransports(ctx, lis)
	}
}

func (tm *Manager) cleanupTransports(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			tm.mx.Lock()
			var toDelete []*ManagedTransport
			for _, tp := range tm.tps {
				if tp.IsClosed() {
					toDelete = append(toDelete, tp)
				}
			}
			for _, tp := range toDelete {
				delete(tm.tps, tp.Entry.ID)
			}
			tm.mx.Unlock()
			if len(toDelete) > 0 {
				tm.Logger.Infof("Deleted %d unused transport entries", len(toDelete))
			}
		case <-ctx.Done():
			return
		}
	}
}

func (tm *Manager) acceptTransports(ctx context.Context, lis network.Listener) {
	defer tm.wg.Done()
	for {
		select {
		case <-ctx.Done():
		case <-tm.done:
			return
		default:
			if err := tm.acceptTransport(ctx, lis); err != nil {
				tm.Logger.Warnf("Failed to accept connection: %v", err)
				if errors.Is(err, io.ErrClosedPipe) {
					return
				}
			}
		}
	}
}

// Networks returns all the network types contained within the TransportManager.
func (tm *Manager) Networks() []string {
	var nets []string
	for netType := range tm.netClients {
		nets = append(nets, string(netType))
	}
	return nets
}

// Stcpr returns stcpr client
func (tm *Manager) Stcpr() (network.Client, bool) {
	c, ok := tm.netClients[network.STCP]
	return c, ok
}

func (tm *Manager) acceptTransport(ctx context.Context, lis network.Listener) error {
	conn, err := lis.AcceptConn() // TODO: tcp panic.
	if err != nil {
		return err
	}

	tm.Logger.Infof("recv transport connection request: type(%s) remote(%s)", lis.Network(), conn.RemotePK())

	tm.mx.Lock()
	defer tm.mx.Unlock()

	if tm.isClosing() {
		return errors.New("transport.Manager is closing. Skipping incoming transport")
	}

	// For transports for purpose(data).

	tpID := tm.tpIDFromPK(conn.RemotePK(), conn.Network())

	client, ok := tm.netClients[network.Type(conn.Network())]
	if !ok {
		return fmt.Errorf("client not found for the type %s", conn.Network())
	}

	mTp, ok := tm.tps[tpID]
	if !ok {
		tm.Logger.Debugln("No TP found, creating new one")

		mTp = NewManagedTransport(ManagedTransportConfig{
			client:         client,
			DC:             tm.Conf.DiscoveryClient,
			LS:             tm.Conf.LogStore,
			RemotePK:       conn.RemotePK(),
			TransportLabel: LabelUser,
			ebc:            tm.ebc,
		})

		go func() {
			mTp.Serve(tm.readCh)

			tm.mx.Lock()
			delete(tm.tps, mTp.Entry.ID)
			tm.mx.Unlock()
		}()

		tm.tps[tpID] = mTp
	} else {
		tm.Logger.Debugln("TP found, accepting...")
	}

	if err := mTp.Accept(ctx, conn); err != nil {
		return err
	}

	tm.Logger.Infof("accepted tp: type(%s) remote(%s) tpID(%s) new(%v)", lis.Network(), conn.RemotePK(), tpID, !ok)

	return nil
}

// ErrNotFound is returned when requested transport is not found
var ErrNotFound = errors.New("transport not found")

// ErrUnknownNetwork occurs on attempt to use an unknown network type.
var ErrUnknownNetwork = errors.New("unknown network type")

// IsKnownNetwork returns true when netName is a known
// network type that we are able to operate in
func (tm *Manager) IsKnownNetwork(netName network.Type) bool {
	_, ok := tm.netClients[netName]
	return ok
}

// GetTransport gets transport entity to the given remote
func (tm *Manager) GetTransport(remote cipher.PubKey, netType network.Type) (*ManagedTransport, error) {
	tm.mx.RLock()
	defer tm.mx.RUnlock()
	if !tm.IsKnownNetwork(netType) {
		return nil, ErrUnknownNetwork
	}

	tpID := tm.tpIDFromPK(remote, netType)
	tp, ok := tm.tps[tpID]
	if !ok {
		return nil, fmt.Errorf("transport to %s of type %s error: %w", remote, netType, ErrNotFound)
	}
	return tp, nil
}

// GetTransportByID retrieves transport by its ID, if it exists
func (tm *Manager) GetTransportByID(tpID uuid.UUID) (*ManagedTransport, error) {
	tp, ok := tm.tps[tpID]
	if !ok {
		return nil, ErrNotFound
	}
	return tp, nil
}

// GetTransportsByLabel returns all transports that have given label
func (tm *Manager) GetTransportsByLabel(label Label) []*ManagedTransport {
	tm.mx.RLock()
	defer tm.mx.RUnlock()
	var trs []*ManagedTransport
	for _, tr := range tm.tps {
		if tr.Entry.Label == label {
			trs = append(trs, tr)
		}
	}
	return trs
}

// SaveTransport begins to attempt to establish data transports to the given 'remote' visor.
func (tm *Manager) SaveTransport(ctx context.Context, remote cipher.PubKey, netType network.Type, label Label) (*ManagedTransport, error) {
	if tm.isClosing() {
		return nil, io.ErrClosedPipe
	}
	for {
		mTp, err := tm.saveTransport(ctx, remote, netType, label)

		if err != nil {
			if err == ErrNotServing {
				continue
			}
			return nil, fmt.Errorf("save transport: %w", err)
		}
		return mTp, nil
	}
}

func (tm *Manager) saveTransport(ctx context.Context, remote cipher.PubKey, netType network.Type, label Label) (*ManagedTransport, error) {
	tm.mx.Lock()
	defer tm.mx.Unlock()
	if !tm.IsKnownNetwork(netType) {
		return nil, ErrUnknownNetwork
	}

	tpID := tm.tpIDFromPK(remote, netType)
	tm.Logger.Debugf("Initializing TP with ID %s", tpID)

	oldMTp, ok := tm.tps[tpID]
	if ok {
		tm.Logger.Debug("Found an old mTp from internal map.")
		return oldMTp, nil
	}

	client, ok := tm.netClients[network.Type(netType)]
	if !ok {
		return nil, fmt.Errorf("client not found for the type %s", netType)
	}

	mTp := NewManagedTransport(ManagedTransportConfig{
		client:         client,
		ebc:            tm.ebc,
		DC:             tm.Conf.DiscoveryClient,
		LS:             tm.Conf.LogStore,
		RemotePK:       remote,
		TransportLabel: label,
	})

	tm.Logger.Debugf("Dialing transport to %v via %v", mTp.Remote(), mTp.client.Type())
	if err := mTp.Dial(ctx); err != nil {
		tm.Logger.Debugf("Error dialing transport to %v via %v: %v", mTp.Remote(), mTp.client.Type(), err)
		if closeErr := mTp.Close(); closeErr != nil {
			tm.Logger.WithError(err).Warn("Error closing transport")
		}
		return nil, err
	}
	go mTp.Serve(tm.readCh)
	tm.tps[tpID] = mTp
	tm.Logger.Infof("saved transport: remote(%s) type(%s) tpID(%s)", remote, netType, tpID)
	return mTp, nil
}

// STCPRRemoteAddrs gets remote IPs for all known STCPR transports.
func (tm *Manager) STCPRRemoteAddrs() []string {
	var addrs []string

	tm.mx.RLock()
	defer tm.mx.RUnlock()

	for _, tp := range tm.tps {
		remoteRaw := tp.conn.RemoteRawAddr().String()
		if tp.Entry.Type == network.STCPR && remoteRaw != "" {
			addrs = append(addrs, remoteRaw)
		}
	}

	return addrs
}

// DeleteTransport deregisters the Transport of Transport ID in transport discovery and deletes it locally.
func (tm *Manager) DeleteTransport(id uuid.UUID) {
	tm.mx.Lock()
	defer tm.mx.Unlock()

	if tm.isClosing() {
		return
	}

	// Deregister transport before closing the underlying connection.
	if tp, ok := tm.tps[id]; ok {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		// todo: this should probably be moved to tp.close because we want to deregister
		// a transport completely and not deal with transport statuses at all
		if err := tm.Conf.DiscoveryClient.DeleteTransport(ctx, id); err != nil {
			tm.Logger.WithError(err).Warnf("Failed to deregister transport of ID %s from discovery.", id)
		} else {
			tm.Logger.Infof("De-registered transport of ID %s from discovery.", id)
		}

		// Close underlying connection.
		tp.close()
		delete(tm.tps, id)
	}
}

// ReadPacket reads data packets from routes.
func (tm *Manager) ReadPacket() (routing.Packet, error) {
	p, ok := <-tm.readCh
	if !ok {
		return nil, ErrNotServing
	}
	return p, nil
}

/*
	STATE
*/

// Transport obtains a Transport via a given Transport ID.
func (tm *Manager) Transport(id uuid.UUID) *ManagedTransport {
	tm.mx.RLock()
	tr := tm.tps[id]
	tm.mx.RUnlock()
	return tr
}

// WalkTransports ranges through all transports.
func (tm *Manager) WalkTransports(walk func(tp *ManagedTransport) bool) {
	tm.mx.RLock()
	for _, tp := range tm.tps {
		if ok := walk(tp); !ok {
			break
		}
	}
	tm.mx.RUnlock()
}

// Local returns Manager.config.PubKey
func (tm *Manager) Local() cipher.PubKey {
	return tm.Conf.PubKey
}

// Close closes opened transports and registered factories.
func (tm *Manager) Close() error {
	tm.closeOnce.Do(tm.close)
	return nil
}

func (tm *Manager) close() {
	tm.Logger.Info("transport manager is closing.")
	defer tm.Logger.Info("transport manager closed.")

	tm.mx.Lock()
	defer tm.mx.Unlock()

	close(tm.done)

	for _, tr := range tm.tps {
		tr.close()
	}

	tm.wgMu.Lock()
	tm.wg.Wait()
	tm.wgMu.Unlock()

	close(tm.readCh)
}

func (tm *Manager) isClosing() bool {
	select {
	case <-tm.done:
		return true
	default:
		return false
	}
}

func (tm *Manager) tpIDFromPK(pk cipher.PubKey, netType network.Type) uuid.UUID {
	return MakeTransportID(tm.Conf.PubKey, pk, netType)
}
