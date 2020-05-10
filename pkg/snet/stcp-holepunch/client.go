package stcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/libp2p/go-reuseport"

	"github.com/SkycoinProject/skywire-mainnet/pkg/snet/stcp/arclient"
)

// Type is stcp type.
const Type = "stcp"

// Client is the central control for incoming and outgoing 'stcp.Conn's.
type Client struct {
	log *logging.Logger

	lPK             cipher.PubKey
	lSK             cipher.SecKey
	p               *Porter
	addressResolver arclient.APIClient

	lTCP net.Listener
	lMap map[uint16]*Listener // key: lPort
	mx   sync.Mutex

	done chan struct{}
	once sync.Once
}

// NewClient creates a net Client.
func NewClient(pk cipher.PubKey, sk cipher.SecKey, addressResolverURL string) (*Client, error) {
	addressResolver, err := arclient.NewHTTP(addressResolverURL, pk, sk)
	if err != nil {
		return nil, err
	}

	c := &Client{
		log:             logging.MustGetLogger(Type),
		lPK:             pk,
		lSK:             sk,
		addressResolver: addressResolver,
		p:               newPorter(PorterMinEphemeral),
		lMap:            make(map[uint16]*Listener),
		done:            make(chan struct{}),
	}

	return c, nil
}

// SetLogger sets a logger for Client.
func (c *Client) SetLogger(log *logging.Logger) {
	c.log = log
}

// Serve serves the listening portion of the client.
func (c *Client) Serve(tcpAddr string) error {
	if c.lTCP != nil {
		return errors.New("already listening")
	}

	c.log.Debugf("reuseport.Listen: %v", tcpAddr)
	lTCP, err := reuseport.Listen("tcp4", tcpAddr)
	if err != nil {
		return err
	}

	c.lTCP = lTCP

	localAddr := lTCP.Addr()
	c.log.Infof("listening on tcp addr: %v", localAddr)

	transport := &http.Transport{
		DialContext: func(_ context.Context, network, addr string) (conn net.Conn, err error) {
			for i := 1; i <= 100; i++ {
				c.log.WithField("attempt", i).Infof("[reuseport.Dial] Trying to connect to %v via %v", addr, network)
				conn, err = reuseport.Dial(network, localAddr.String(), addr)
				if err == nil {
					break
				}

				const delay = 100 * time.Millisecond
				c.log.WithError(err).Warnf("[reuseport.Dial] Failed to establish connection to %v via %v, waiting %v", addr, network, delay)
				time.Sleep(delay)
			}
			c.log.Infof("[reuseport.Dial] Established connection to %v via %v", addr, network)
			return conn, err
		},
		DisableKeepAlives: false,
	}

	c.addressResolver.SetTransport(transport)

	//_, port, err := net.SplitHostPort(localAddr.String())
	//if err != nil {
	//	port = ""
	//}

	//if err := c.addressResolver.Bind(context.Background(), port); err != nil {
	//	return fmt.Errorf("bind PK")
	//}

	if err := c.addressResolver.Bind(context.Background(), ""); err != nil {
		return fmt.Errorf("bind PK")
	}

	go func() {
		for {
			if err := c.acceptTCPConn(); err != nil {
				c.log.Warnf("failed to accept incoming connection: %v", err)

				if !IsHandshakeError(err) {
					c.log.Warnf("stopped serving stcp")
					return
				}
			}
		}
	}()

	return nil
}

func (c *Client) acceptTCPConn() error {
	if c.isClosed() {
		return io.ErrClosedPipe
	}

	c.log.Debugf("Accepting conn on %v", c.lTCP.Addr())
	tcpConn, err := c.lTCP.Accept()
	if err != nil {
		return fmt.Errorf("lTCP.Accept: %w", err)
	}

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

	conn, err := newConn(tcpConn, time.Now().Add(HandshakeTimeout), hs, nil)
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

	addr, err := c.addressResolver.Resolve(ctx, rPK)
	if err != nil {
		return nil, err
	}

	c.log.Debugf("PK %v resolved to address %v", rPK, addr)

	var netConn net.Conn
	for i := 1; i <= 100; i++ {
		c.log.Debugf("Dialing tcp address %v attempt %v", addr, i)

		var err error
		netConn, err = net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			break
		}

		if i == 100 {
			return nil, fmt.Errorf("net.Dial: %w", err)
		}
		// TODO: websocket signaling
		c.log.Errorf("Failed to dial %v: %v. Retrying...", addr, err)
		time.Sleep(500 * time.Millisecond)
	}

	lPort, freePort, err := c.p.ReserveEphemeral(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReserveEphemeral: %w", err)
	}

	hs := InitiatorHandshake(c.lSK, dmsg.Addr{PK: c.lPK, Port: lPort}, dmsg.Addr{PK: rPK, Port: rPort})

	stcpConn, err := newConn(netConn, time.Now().Add(HandshakeTimeout), hs, freePort)
	if err != nil {
		return nil, fmt.Errorf("newConn: %w", err)
	}

	return stcpConn, nil
}

// Listen creates a new listener for stcp.
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

		if c.lTCP != nil {
			_ = c.lTCP.Close() //nolint:errcheck
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
