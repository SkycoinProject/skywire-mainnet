package network

import (
	"fmt"
	"net"
	"time"

	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/noise"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/pkg/transport/network/handshake"
)

const encryptHSTimout = 5 * time.Second

// Conn represents a network connection between two visors in skywire network
// This connection wraps raw network connection and is ready to use for sending data.
// It also provides skywire-specific methods on top of net.Conn
type Conn interface {
	net.Conn
	// LocalPK returns local public key of connection
	LocalPK() cipher.PubKey

	// RemotePK returns remote public key of connection
	RemotePK() cipher.PubKey

	// LocalPort returns local skywire port of connection
	// This is not underlying OS port, but port within skywire network
	LocalPort() uint16

	// RemotePort returns remote skywire port of connection
	// This is not underlying OS port, but port within skywire network
	RemotePort() uint16

	// LocalRawAddr returns local raw network address (not skywire address)
	LocalRawAddr() net.Addr

	// RemoteRawAddr returns remote raw network address (not skywire address)
	RemoteRawAddr() net.Addr

	// Network returns network of connection
	Network() Type
}

type conn struct {
	net.Conn
	lAddr, rAddr dmsg.Addr
	freePort     func()
	connType     Type
}

// DoHandshake performs given handshake over given raw connection and wraps
// connection in network.Conn
func DoHandshake(rawConn net.Conn, hs handshake.Handshake, netType Type, log *logging.Logger) (Conn, error) {
	return doHandshake(rawConn, hs, netType, log)
}

// handshake performs given handshake over given raw connection and wraps
// connection in network.conn
func doHandshake(rawConn net.Conn, hs handshake.Handshake, netType Type, log *logging.Logger) (*conn, error) {
	lAddr, rAddr, err := hs(rawConn, time.Now().Add(handshake.Timeout))
	if err != nil {
		if err := rawConn.Close(); err != nil {
			log.WithError(err).Warnf("Failed to close connection")
		}
		return nil, err
	}
	handshakedConn := &conn{Conn: rawConn, lAddr: lAddr, rAddr: rAddr, connType: netType}
	return handshakedConn, nil
}

func (c *conn) encrypt(lPK cipher.PubKey, lSK cipher.SecKey, initator bool) error {
	config := noise.Config{
		LocalPK:   lPK,
		LocalSK:   lSK,
		RemotePK:  c.rAddr.PK,
		Initiator: initator,
	}

	wrappedConn, err := EncryptConn(config, c.Conn)
	if err != nil {
		return fmt.Errorf("encrypt connection to %v@%v: %w", c.rAddr, c.Conn.RemoteAddr(), err)
	}

	c.Conn = wrappedConn
	return nil
}

// EncryptConn encrypts given connection
func EncryptConn(config noise.Config, conn net.Conn) (net.Conn, error) {
	ns, err := noise.New(noise.HandshakeKK, config)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stream noise object: %w", err)
	}

	wrappedConn, err := noise.WrapConn(conn, ns, encryptHSTimout)
	if err != nil {
		return nil, fmt.Errorf("error performing noise handshake: %w", err)
	}

	return wrappedConn, nil
}

// LocalAddr implements net.Conn
func (c *conn) LocalAddr() net.Addr {
	return c.lAddr
}

// RemoteAddr implements net.Conn
func (c *conn) RemoteAddr() net.Addr {
	return c.rAddr
}

// LocalAddr implements net.Conn
func (c *conn) LocalRawAddr() net.Addr {
	return c.Conn.LocalAddr()
}

// RemoteAddr implements net.Conn
func (c *conn) RemoteRawAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

// Close implements net.Conn
func (c *conn) Close() error {
	if c.freePort != nil {
		c.freePort()
	}

	return c.Conn.Close()
}

// LocalPK returns local public key of connection
func (c *conn) LocalPK() cipher.PubKey { return c.lAddr.PK }

// RemotePK returns remote public key of connection
func (c *conn) RemotePK() cipher.PubKey { return c.rAddr.PK }

// LocalPort returns local skywire port of connection
// This is not underlying OS port, but port within skywire network
func (c *conn) LocalPort() uint16 { return c.lAddr.Port }

// RemotePort returns remote skywire port of connection
// This is not underlying OS port, but port within skywire network
func (c *conn) RemotePort() uint16 { return c.rAddr.Port }

// Network returns network of connection
func (c *conn) Network() Type { return c.connType }
