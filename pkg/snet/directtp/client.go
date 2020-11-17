package directtp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/AudriusButkevicius/pfilter"
	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"
	"github.com/xtaci/kcp-go"

	"github.com/skycoin/skywire/internal/packetfilter"
	"github.com/skycoin/skywire/pkg/snet/arclient"
	"github.com/skycoin/skywire/pkg/snet/directtp/pktable"
	"github.com/skycoin/skywire/pkg/snet/directtp/porter"
	"github.com/skycoin/skywire/pkg/snet/directtp/tpconn"
	"github.com/skycoin/skywire/pkg/snet/directtp/tphandshake"
	"github.com/skycoin/skywire/pkg/snet/directtp/tplistener"
	"github.com/skycoin/skywire/pkg/snet/directtp/tptypes"
)

const (
	// holePunchMessage is sent in a dummy UDP packet that is sent by both parties to establish UDP hole punching.
	holePunchMessage = "holepunch"
	dialTimeout      = 30 * time.Second
	// dialConnPriority and visorsConnPriority are used to set an order how connection filters apply.
	dialConnPriority   = 2
	visorsConnPriority = 3
)

var (
	// ErrUnknownTransportType is returned when transport type is unknown.
	ErrUnknownTransportType = errors.New("unknown transport type")

	// ErrTimeout indicates a timeout.
	ErrTimeout = errors.New("timeout")

	// ErrAlreadyListening is returned when transport is already listening.
	ErrAlreadyListening = errors.New("already listening")

	// ErrNotListening is returned when transport is not listening.
	ErrNotListening = errors.New("not listening")

	// ErrPortOccupied is returned when port is occupied.
	ErrPortOccupied = errors.New("port is already occupied")
)

// Client is the central control for incoming and outgoing 'Conn's.
type Client interface {
	Dial(ctx context.Context, rPK cipher.PubKey, rPort uint16) (*tpconn.Conn, error)
	Listen(lPort uint16) (*tplistener.Listener, error)
	LocalAddr() (net.Addr, error)
	Serve(ctx context.Context) error
	Close() error
	Type() string
}

// Config configures Client.
type Config struct {
	Type               string
	PK                 cipher.PubKey
	SK                 cipher.SecKey
	LocalAddr          string
	Table              pktable.PKTable
	AddressResolver    arclient.APIClient
	BeforeDialCallback BeforeDialCallback
}

// BeforeDialCallback is triggered before client dials.
// If a non-nil error is returned, the dial is instantly terminated.
type BeforeDialCallback func(network, addr string) (err error)

type client struct {
	conf               Config
	mu                 sync.Mutex
	done               chan struct{}
	once               sync.Once
	log                *logging.Logger
	porter             *porter.Porter
	listener           net.Listener
	listening          chan struct{}
	listeners          map[uint16]*tplistener.Listener // key: lPort
	sudphPacketFilter  *pfilter.PacketFilter
	sudphListener      net.PacketConn
	sudphVisorsConn    net.PacketConn
	beforeDialCallback BeforeDialCallback
}

// NewClient creates a net Client.
func NewClient(conf Config) Client {
	return &client{
		conf:               conf,
		log:                logging.MustGetLogger(conf.Type),
		porter:             porter.New(porter.MinEphemeral),
		listeners:          make(map[uint16]*tplistener.Listener),
		done:               make(chan struct{}),
		listening:          make(chan struct{}),
		beforeDialCallback: conf.BeforeDialCallback,
	}
}

// Serve serves the listening portion of the client.
func (c *client) Serve(ctx context.Context) error {
	switch c.conf.Type {
	case tptypes.STCP, tptypes.STCPR:
		if c.listener != nil {
			return ErrAlreadyListening
		}
	case tptypes.SUDPH:
		if c.sudphListener != nil {
			return ErrAlreadyListening
		}
	}

	go func() {
		l, err := c.listen(c.conf.LocalAddr)
		if err != nil {
			c.log.Errorf("Failed to listen on %q: %v", c.conf.LocalAddr, err)
			return
		}

		c.listener = l
		close(c.listening)

		if c.conf.Type == tptypes.STCPR {
			localAddr := c.listener.Addr().String()
			_, port, err := net.SplitHostPort(localAddr)
			if err != nil {
				c.log.Errorf("Failed to extract port from addr %v: %v", err)
				return
			}

			if err := c.conf.AddressResolver.BindSTCPR(ctx, port); err != nil {
				c.log.Errorf("Failed to bind STCPR: %v", err)
				return
			}
		}

		c.log.Infof("listening on addr: %v", c.listener.Addr())

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := c.acceptConn(); err != nil {
				if strings.Contains(err.Error(), io.EOF.Error()) {
					continue // likely it's a dummy connection from service discovery
				}

				c.log.Warnf("failed to accept incoming connection: %v", err)

				if !tphandshake.IsHandshakeError(err) {
					c.log.Warnf("stopped serving")
					return
				}
			}
		}
	}()

	return nil
}

func (c *client) LocalAddr() (net.Addr, error) {
	<-c.listening

	switch c.conf.Type {
	case tptypes.STCP, tptypes.STCPR:
		if c.listener == nil {
			return nil, ErrNotListening
		}

		return c.listener.Addr(), nil
	case tptypes.SUDPH:
		if c.sudphListener == nil {
			return nil, ErrNotListening
		}

		return c.listener.Addr(), nil
	}

	return nil, ErrUnknownTransportType
}

func (c *client) acceptConn() error {
	if c.isClosed() {
		return io.ErrClosedPipe
	}

	conn, err := c.listener.Accept()
	if err != nil {
		return err
	}

	remoteAddr := conn.RemoteAddr()

	c.log.Infof("Accepted connection from %v", remoteAddr)

	var lis *tplistener.Listener

	hs := tphandshake.ResponderHandshake(func(f2 tphandshake.Frame2) error {
		c.mu.Lock()
		defer c.mu.Unlock()

		var ok bool
		if lis, ok = c.listeners[f2.DstAddr.Port]; !ok {
			return errors.New("not listening on given port")
		}

		return nil
	})

	connConfig := tpconn.Config{
		Log:       c.log,
		Conn:      conn,
		LocalPK:   c.conf.PK,
		LocalSK:   c.conf.SK,
		Deadline:  time.Now().Add(tphandshake.Timeout),
		Handshake: hs,
		FreePort:  nil,
		Encrypt:   true,
		Initiator: false,
	}

	wrappedConn, err := tpconn.NewConn(connConfig)
	if err != nil {
		return err
	}

	if err := lis.Introduce(wrappedConn); err != nil {
		return err
	}

	return nil
}

// Dial dials a new Conn to specified remote public key and port.
func (c *client) Dial(ctx context.Context, rPK cipher.PubKey, rPort uint16) (*tpconn.Conn, error) {
	if c.isClosed() {
		return nil, io.ErrClosedPipe
	}

	c.log.Infof("Dialing PK %v", rPK)

	var visorConn net.Conn

	switch c.conf.Type {
	case tptypes.STCP:
		addr, ok := c.conf.Table.Addr(rPK)
		if !ok {
			return nil, fmt.Errorf("pk table: entry of %s does not exist", rPK)
		}

		conn, err := c.dial(addr)
		if err != nil {
			return nil, err
		}

		visorConn = conn

	case tptypes.STCPR, tptypes.SUDPH:
		visorData, err := c.conf.AddressResolver.Resolve(ctx, c.Type(), rPK)
		if err != nil {
			return nil, fmt.Errorf("resolve PK: %w", err)
		}

		c.log.Infof("Resolved PK %v to visor data %v", rPK, visorData)

		conn, err := c.dialVisor(visorData)
		if err != nil {
			return nil, err
		}

		visorConn = conn

	default:
		return nil, ErrUnknownTransportType
	}

	c.log.Infof("Dialed %v:%v@%v", rPK, rPort, visorConn.RemoteAddr())

	lPort, freePort, err := c.porter.ReserveEphemeral(ctx)
	if err != nil {
		return nil, err
	}

	hs := tphandshake.InitiatorHandshake(c.conf.SK, dmsg.Addr{PK: c.conf.PK, Port: lPort}, dmsg.Addr{PK: rPK, Port: rPort})

	connConfig := tpconn.Config{
		Log:       c.log,
		Conn:      visorConn,
		LocalPK:   c.conf.PK,
		LocalSK:   c.conf.SK,
		Deadline:  time.Now().Add(tphandshake.Timeout),
		Handshake: hs,
		FreePort:  freePort,
		Encrypt:   true,
		Initiator: true,
	}

	return tpconn.NewConn(connConfig)
}

func (c *client) dial(addr string) (net.Conn, error) {
	switch c.conf.Type {
	case tptypes.STCP, tptypes.STCPR:
		return net.Dial("tcp", addr)

	case tptypes.SUDPH:
		return c.dialUDPWithTimeout(addr)

	default:
		return nil, ErrUnknownTransportType
	}
}

func (c *client) listen(addr string) (net.Listener, error) {
	switch c.conf.Type {
	case tptypes.STCP, tptypes.STCPR:
		return net.Listen("tcp", addr)

	case tptypes.SUDPH:
		packetListener, err := net.ListenPacket("udp", "")
		if err != nil {
			return nil, err
		}

		c.sudphListener = packetListener

		c.sudphPacketFilter = pfilter.NewPacketFilter(packetListener)
		c.sudphVisorsConn = c.sudphPacketFilter.NewConn(visorsConnPriority, nil)

		c.sudphPacketFilter.Start()

		addrCh, err := c.conf.AddressResolver.BindSUDPH(c.sudphPacketFilter)
		if err != nil {
			return nil, err
		}

		go func() {
			for addr := range addrCh {
				udpAddr, err := net.ResolveUDPAddr("udp", addr.Addr)
				if err != nil {
					c.log.WithError(err).Errorf("Failed to resolve UDP address %q", addr)
					continue
				}

				c.log.Infof("Sending hole punch packet to %v", addr)

				if _, err := c.sudphVisorsConn.WriteTo([]byte(holePunchMessage), udpAddr); err != nil {
					c.log.WithError(err).Errorf("Failed to send hole punch packet to %v", udpAddr)
					continue
				}

				c.log.Infof("Sent hole punch packet to %v", addr)
			}
		}()

		return kcp.ServeConn(nil, 0, 0, c.sudphVisorsConn)

	default:
		return nil, ErrUnknownTransportType
	}
}

func (c *client) dialUDP(remoteAddr string) (net.Conn, error) {
	rAddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("net.ResolveUDPAddr (remote): %w", err)
	}

	dialConn := c.sudphPacketFilter.NewConn(dialConnPriority, packetfilter.NewKCPConversationFilter())

	if _, err := dialConn.WriteTo([]byte(holePunchMessage), rAddr); err != nil {
		return nil, fmt.Errorf("dialConn.WriteTo: %w", err)
	}

	kcpConn, err := kcp.NewConn(remoteAddr, nil, 0, 0, dialConn)
	if err != nil {
		return nil, err
	}

	return kcpConn, nil
}

func (c *client) dialUDPWithTimeout(addr string) (net.Conn, error) {
	timer := time.NewTimer(dialTimeout)
	defer timer.Stop()

	c.log.Infof("Dialing %v", addr)

	for {
		select {
		case <-timer.C:
			return nil, ErrTimeout
		default:
			conn, err := c.dialUDP(addr)
			if err == nil {
				c.log.Infof("Dialed %v", addr)
				return conn, nil
			}

			c.log.WithError(err).
				Warnf("Failed to dial %v, trying again: %v", addr, err)
		}
	}
}

func (c *client) dialVisor(visorData arclient.VisorData) (net.Conn, error) {
	if visorData.IsLocal {
		for _, host := range visorData.Addresses {
			addr := net.JoinHostPort(host, visorData.Port)

			if c.beforeDialCallback != nil {
				if err := c.beforeDialCallback(c.conf.Type, addr); err != nil {
					return nil, err
				}
			}

			conn, err := c.dial(addr)
			if err == nil {
				return conn, nil
			}
		}
	}

	addr := visorData.RemoteAddr
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, visorData.Port)
	}

	if c.beforeDialCallback != nil {
		if err := c.beforeDialCallback(c.conf.Type, addr); err != nil {
			return nil, err
		}
	}

	return c.dial(addr)
}

// Listen creates a new listener for sudp.
// The created Listener cannot actually accept remote connections unless Serve is called beforehand.
func (c *client) Listen(lPort uint16) (*tplistener.Listener, error) {
	if c.isClosed() {
		return nil, io.ErrClosedPipe
	}

	ok, freePort := c.porter.Reserve(lPort)
	if !ok {
		return nil, ErrPortOccupied
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	lAddr := dmsg.Addr{PK: c.conf.PK, Port: lPort}
	lis := tplistener.NewListener(lAddr, freePort)
	c.listeners[lPort] = lis

	return lis, nil
}

// Close closes the Client.
func (c *client) Close() error {
	if c == nil {
		return nil
	}

	c.once.Do(func() {
		close(c.done)

		c.mu.Lock()
		defer c.mu.Unlock()

		if c.listener != nil {
			if err := c.listener.Close(); err != nil {
				c.log.WithError(err).Warnf("Failed to close listener")
			}
		}

		for _, lis := range c.listeners {
			if err := lis.Close(); err != nil {
				c.log.WithError(err).Warnf("Failed to close listener")
			}
		}

		switch c.Type() {
		case tptypes.STCPR, tptypes.SUDPH:
			if err := c.conf.AddressResolver.Close(); err != nil {
				c.log.WithError(err).Warnf("Failed to close address-resolver")
			}
		}

		if c.sudphVisorsConn != nil {
			if err := c.sudphVisorsConn.Close(); err != nil {
				c.log.WithError(err).Warnf("Failed to close connection to visors")
			}
		}
	})

	return nil
}

func (c *client) isClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Type returns the stream type.
func (c *client) Type() string {
	return c.conf.Type
}
