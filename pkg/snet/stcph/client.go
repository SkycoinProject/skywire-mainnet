package stcph

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/libp2p/go-reuseport"

	"github.com/SkycoinProject/skywire-mainnet/pkg/snet/arclient"
)

// Type is stcp hole punch type.
const Type = "stcph"

// DialTimeout represents a timeout for dialing.
// TODO: Find best value.
const DialTimeout = 30 * time.Second

// ErrTimeout indicates a timeout.
var ErrTimeout = errors.New("timeout")

// Client is the central control for incoming and outgoing 'stcp.Conn's.
type Client struct {
	log *logging.Logger

	lPK             cipher.PubKey
	lSK             cipher.SecKey
	p               *Porter
	addressResolver arclient.APIClient

	connCh <-chan arclient.RemoteVisor
	dialCh chan cipher.PubKey
	lMap   map[uint16]*Listener // key: lPort
	mx     sync.Mutex

	done chan struct{}
	once sync.Once
}

// NewClient creates a net Client.
func NewClient(pk cipher.PubKey, sk cipher.SecKey, addressResolver arclient.APIClient) *Client {
	c := &Client{
		log:             logging.MustGetLogger(Type),
		lPK:             pk,
		lSK:             sk,
		addressResolver: addressResolver,
		p:               newPorter(PorterMinEphemeral),
		lMap:            make(map[uint16]*Listener),
		done:            make(chan struct{}),
	}

	return c
}

// SetLogger sets a logger for Client.
func (c *Client) SetLogger(log *logging.Logger) {
	c.log = log
}

// Serve serves the listening portion of the client.
func (c *Client) Serve() error {
	if c.connCh != nil {
		return errors.New("already listening")
	}

	ctx := context.Background()

	dialCh := make(chan cipher.PubKey)

	connCh, err := c.addressResolver.WS(ctx, dialCh)
	if err != nil {
		return err
	}

	c.connCh = connCh
	c.dialCh = dialCh

	c.log.Infof("listening websocket events on %v", c.addressResolver.LocalAddr())

	go func() {
		for addr := range c.connCh {
			c.log.Infof("Received signal to dial %v", addr)

			go func(addr arclient.RemoteVisor) {
				if err := c.acceptTCPConn(addr); err != nil {
					c.log.Warnf("failed to accept incoming connection: %v", err)
				}
			}(addr)
		}
	}()

	return nil
}

func (c *Client) dialTimeout(addr string) (net.Conn, error) {
	timer := time.NewTimer(DialTimeout)
	defer timer.Stop()

	c.log.Infof("Dialing %v from %v via tcp", addr, c.addressResolver.LocalAddr())

	for {
		select {
		case <-timer.C:
			return nil, ErrTimeout
		default:
			conn, err := reuseport.Dial("tcp", c.addressResolver.LocalAddr(), addr)
			if err == nil {
				c.log.Infof("Dialed %v from %v", addr, c.addressResolver.LocalAddr())
				return conn, nil
			}

			c.log.WithError(err).
				Warnf("Failed to dial %v from %v, trying again: %v", addr, c.addressResolver.LocalAddr(), err)
		}
	}
}

func (c *Client) acceptTCPConn(remote arclient.RemoteVisor) error {
	if c.isClosed() {
		return io.ErrClosedPipe
	}

	tcpConn, err := c.dialTimeout(remote.Addr)
	if err != nil {
		return err
	}

	remoteAddr := tcpConn.RemoteAddr()

	c.log.Infof("Accepted connection from %v", remoteAddr)

	var lis *Listener

	hs := ResponderHandshake(func(f2 Frame2) error {
		c.mx.Lock()
		defer c.mx.Unlock()

		var ok bool
		if lis, ok = c.lMap[f2.DstAddr.Port]; !ok {
			return errors.New("not listening on given port")
		}

		return nil
	})

	connConfig := connConfig{
		log:       c.log,
		conn:      tcpConn,
		localPK:   c.lPK,
		localSK:   c.lSK,
		deadline:  time.Now().Add(HandshakeTimeout),
		hs:        hs,
		freePort:  nil,
		encrypt:   true,
		initiator: false,
	}

	conn, err := newConn(connConfig)
	if err != nil {
		return err
	}

	return lis.Introduce(conn)
}

// Dial dials a new stcp.Conn to specified remote public key and port.
func (c *Client) Dial(ctx context.Context, rPK cipher.PubKey, rPort uint16) (*Conn, error) {
	if c.isClosed() {
		return nil, io.ErrClosedPipe
	}

	c.log.Infof("Dialing PK %v", rPK)

	c.dialCh <- rPK

	addr, err := c.addressResolver.ResolveHolePunch(ctx, rPK)
	if err != nil {
		return nil, err
	}

	c.log.Infof("Resolved PK %v to addr %v, dialing", rPK, addr)

	tcpConn, err := c.dialTimeout(addr)
	if err != nil {
		return nil, err
	}

	c.log.Infof("Dialed %v:%v@%v", rPK, rPort, addr)

	lPort, freePort, err := c.p.ReserveEphemeral(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReserveEphemeral: %w", err)
	}

	hs := InitiatorHandshake(c.lSK, dmsg.Addr{PK: c.lPK, Port: lPort}, dmsg.Addr{PK: rPK, Port: rPort})

	connConfig := connConfig{
		log:       c.log,
		conn:      tcpConn,
		localPK:   c.lPK,
		localSK:   c.lSK,
		deadline:  time.Now().Add(HandshakeTimeout),
		hs:        hs,
		freePort:  freePort,
		encrypt:   true,
		initiator: true,
	}

	stcpConn, err := newConn(connConfig)
	if err != nil {
		return nil, fmt.Errorf("newConn: %w", err)
	}

	return stcpConn, nil
}

// Listen creates a new listener for stcp hole punch.
// The created Listener cannot actually accept remote connections unless Serve is called beforehand.
func (c *Client) Listen(lPort uint16) (*Listener, error) {
	if c.isClosed() {
		return nil, io.ErrClosedPipe
	}

	ok, freePort := c.p.Reserve(lPort)
	if !ok {
		return nil, errors.New("port is already occupied")
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	lAddr := dmsg.Addr{PK: c.lPK, Port: lPort}
	lis := newListener(lAddr, freePort)
	c.lMap[lPort] = lis

	return lis, nil
}

// Close closes the Client.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	c.once.Do(func() {
		close(c.done)

		c.mx.Lock()
		defer c.mx.Unlock()

		if err := c.addressResolver.Close(); err != nil {
			c.log.WithError(err).Warnf("Failed to close address resolver client")
		}

		for _, lis := range c.lMap {
			_ = lis.Close() // nolint:errcheck
		}
	})

	return nil
}

func (c *Client) isClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Type returns the stream type.
func (c *Client) Type() string {
	return Type
}
