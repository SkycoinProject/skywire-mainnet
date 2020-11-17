package vpn

import (
	"io"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

// ClientMobile is a VPN client used for mobile devices.
type ClientMobile struct {
	cfg       ClientConfig
	log       logrus.FieldLogger
	conn      net.Conn
	closeC    chan struct{}
	closeOnce sync.Once
}

// NewClientMobile create VPN client instance to be used on mobile devices.
func NewClientMobile(cfg ClientConfig, l logrus.FieldLogger, conn net.Conn) *ClientMobile {
	return &ClientMobile{
		cfg:    cfg,
		log:    l,
		conn:   conn,
		closeC: make(chan struct{}),
	}
}

// GetConn returns VPN server connection.
func (c *ClientMobile) GetConn() net.Conn {
	return c.conn
}

// Close closes client.
func (c *ClientMobile) Close() {
	c.closeOnce.Do(func() {
		close(c.closeC)
	})
}

// ShakeHands performs client/server handshake.
func (c *ClientMobile) ShakeHands() (TUNIP, TUNGateway net.IP, err error) {
	cHello := ClientHello{
		Passcode: c.cfg.Passcode,
	}

	return DoClientHandshake(c.log, c.GetConn(), cHello)
}

// Serve starts handling traffic.
func (c *ClientMobile) Serve(udpConnRW *UDPConnWriter) error {
	tunnelConn := c.GetConn()

	connToUDPDoneCh := make(chan struct{})
	udpToConnCh := make(chan struct{})
	// read all system traffic and pass it to the remote VPN server
	go func() {
		defer close(connToUDPDoneCh)

		if _, err := io.Copy(udpConnRW, tunnelConn); err != nil {
			c.log.WithError(err).Errorln("Error resending traffic from VPN server to mobile app UDP conn")
		}
	}()
	go func() {
		defer close(udpToConnCh)

		if _, err := io.Copy(tunnelConn, udpConnRW.conn); err != nil {
			c.log.WithError(err).Errorln("Error resending traffic from mobile app UDP conn to VPN server")
		}
	}()

	// only one side may fail here, so we wait till at least one fails
	select {
	case <-connToUDPDoneCh:
	case <-udpToConnCh:
	case <-c.closeC:
	}

	return nil
}
