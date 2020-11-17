package vpn

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"
)

// Server is a VPN server.
type Server struct {
	cfg                        ServerConfig
	lisMx                      sync.Mutex
	lis                        net.Listener
	log                        logrus.FieldLogger
	serveOnce                  sync.Once
	ipGen                      *IPGenerator
	defaultNetworkInterface    string
	defaultNetworkInterfaceIPs []net.IP
	ipv4ForwardingVal          string
	ipv6ForwardingVal          string
}

// NewServer creates VPN server instance.
func NewServer(cfg ServerConfig, l logrus.FieldLogger) (*Server, error) {
	s := &Server{
		cfg:   cfg,
		log:   l,
		ipGen: NewIPGenerator(),
	}

	defaultNetworkIfc, err := DefaultNetworkInterface()
	if err != nil {
		return nil, fmt.Errorf("error getting default network interface: %w", err)
	}

	l.Infof("Got default network interface: %s", defaultNetworkIfc)

	defaultNetworkIfcIPs, err := NetworkInterfaceIPs(defaultNetworkIfc)
	if err != nil {
		return nil, fmt.Errorf("error getting IPs of interface %s: %w", defaultNetworkIfc, err)
	}

	l.Infof("Got IPs of interface %s: %v", defaultNetworkIfc, defaultNetworkIfcIPs)

	ipv4ForwardingVal, err := GetIPv4ForwardingValue()
	if err != nil {
		return nil, fmt.Errorf("error getting IPv4 forwarding value: %w", err)
	}
	ipv6ForwardingVal, err := GetIPv6ForwardingValue()
	if err != nil {
		return nil, fmt.Errorf("error getting IPv6 forwarding value")
	}

	l.Infoln("Old IP forwarding values:")
	l.Infof("IPv4: %s, IPv6: %s", ipv4ForwardingVal, ipv6ForwardingVal)

	s.defaultNetworkInterface = defaultNetworkIfc
	s.defaultNetworkInterfaceIPs = defaultNetworkIfcIPs
	s.ipv4ForwardingVal = ipv4ForwardingVal
	s.ipv6ForwardingVal = ipv6ForwardingVal

	return s, nil
}

// Serve accepts connections from `l` and serves them.
func (s *Server) Serve(l net.Listener) error {
	serveErr := errors.New("already serving")
	s.serveOnce.Do(func() {
		if err := EnableIPv4Forwarding(); err != nil {
			serveErr = fmt.Errorf("error enabling IPv4 forwarding: %w", err)
			return
		}
		s.log.Infoln("Set IPv4 forwarding = 1")
		defer func() {
			if err := SetIPv4ForwardingValue(s.ipv4ForwardingVal); err != nil {
				s.log.WithError(err).Errorln("Error reverting IPv4 forwarding")
			} else {
				s.log.Infof("Set IPv4 forwarding = %s", s.ipv4ForwardingVal)
			}
		}()

		if err := EnableIPv6Forwarding(); err != nil {
			serveErr = fmt.Errorf("error enabling IPv6 forwarding: %w", err)
			return
		}
		s.log.Infoln("Set IPv6 forwarding = 1")
		defer func() {
			if err := SetIPv6ForwardingValue(s.ipv6ForwardingVal); err != nil {
				s.log.WithError(err).Errorln("Error reverting IPv6 forwarding")
			} else {
				s.log.Infof("Set IPv6 forwarding = %s", s.ipv6ForwardingVal)
			}
		}()

		if err := EnableIPMasquerading(s.defaultNetworkInterface); err != nil {
			serveErr = fmt.Errorf("error enabling IP masquerading for %s: %w", s.defaultNetworkInterface, err)
			return
		}

		s.log.Infoln("Enabled IP masquerading")

		defer func() {
			if err := DisableIPMasquerading(s.defaultNetworkInterface); err != nil {
				s.log.WithError(err).Errorf("Error disabling IP masquerading for %s", s.defaultNetworkInterface)
			} else {
				s.log.Infof("Disabled IP masquerading for %s", s.defaultNetworkInterface)
			}
		}()

		s.lisMx.Lock()
		s.lis = l
		s.lisMx.Unlock()

		for {
			conn, err := s.lis.Accept()
			if err != nil {
				serveErr = fmt.Errorf("failed to accept client connection: %w", err)
				return
			}

			go s.serveConn(conn)
		}
	})

	return serveErr
}

// Close shuts server down.
func (s *Server) Close() error {
	s.lisMx.Lock()
	defer s.lisMx.Unlock()

	if s.lis == nil {
		return nil
	}

	err := s.lis.Close()
	s.lis = nil

	return err
}

func (s *Server) closeConn(conn net.Conn) {
	if err := conn.Close(); err != nil {
		s.log.WithError(err).Errorf("Error closing client %s connection", conn.RemoteAddr())
	}
}

func (s *Server) serveConn(conn net.Conn) {
	defer s.closeConn(conn)

	tunIP, tunGateway, allowTrafficToLocalNet, err := s.shakeHands(conn)
	if err != nil {
		s.log.WithError(err).Errorf("Error negotiating with client %s", conn.RemoteAddr())
		return
	}
	defer allowTrafficToLocalNet()

	tun, err := newTUNDevice()
	if err != nil {
		s.log.WithError(err).Errorln("Error allocating TUN interface")
		return
	}
	defer func() {
		tunName := tun.Name()
		if err := tun.Close(); err != nil {
			s.log.WithError(err).Errorf("Error closing TUN %s", tunName)
		}
	}()

	s.log.Infof("Allocated TUN %s", tun.Name())

	err = SetupTUN(tun.Name(), tunIP.String()+"/"+strconv.Itoa(TUNNetmaskPrefix), tunGateway.String(), TUNMTU)
	if err != nil {
		s.log.WithError(err).Errorf("Error setting up TUN %s", tun.Name())
		return
	}

	connToTunDoneCh := make(chan struct{})
	tunToConnCh := make(chan struct{})
	go func() {
		defer close(connToTunDoneCh)

		if _, err := io.Copy(tun, conn); err != nil {
			s.log.WithError(err).Errorf("Error resending traffic from VPN client to TUN %s", tun.Name())
		}
	}()
	go func() {
		defer close(tunToConnCh)

		if _, err := io.Copy(conn, tun); err != nil {
			s.log.WithError(err).Errorf("Error resending traffic from TUN %s to VPN client", tun.Name())
		}
	}()

	// only one side may fail here, so we wait till at least one fails
	select {
	case <-connToTunDoneCh:
	case <-tunToConnCh:
	}
}

func (s *Server) shakeHands(conn net.Conn) (tunIP, tunGateway net.IP, unsecureVPN func(), err error) {
	var cHello ClientHello
	if err := ReadJSON(conn, &cHello); err != nil {
		return nil, nil, nil, fmt.Errorf("error reading client hello: %w", err)
	}

	// default value
	unsecureVPN = func() {}

	s.log.Debugf("Got client hello: %v", cHello)

	if s.cfg.Passcode != "" && cHello.Passcode != s.cfg.Passcode {
		s.sendServerErrHello(conn, HandshakeStatusForbidden)
		return nil, nil, nil, errors.New("got wrong passcode from client")
	}

	for _, ip := range cHello.UnavailablePrivateIPs {
		if err := s.ipGen.Reserve(ip); err != nil {
			// this happens only on malformed IP
			s.sendServerErrHello(conn, HandshakeStatusBadRequest)
			return nil, nil, nil, fmt.Errorf("error reserving IP %s: %w", ip.String(), err)
		}
	}

	subnet, err := s.ipGen.Next()
	if err != nil {
		s.sendServerErrHello(conn, HandshakeNoFreeIPs)
		return nil, nil, nil, fmt.Errorf("error getting free subnet IP: %w", err)
	}

	subnetOctets, err := fetchIPv4Octets(subnet)
	if err != nil {
		s.sendServerErrHello(conn, HandshakeStatusInternalError)
		return nil, nil, nil, fmt.Errorf("error breaking IP into octets: %w", err)
	}

	// basically IP address comprised of `subnetOctets` items is the IP address of the subnet,
	// we're going to work with. In this subnet we're giving 4 IP addresses: IP and gateway for
	// the server-side TUN and IP and gateway for the client-side TUN. We do this as follows:
	// - Server-side TUN gateway = subnet IP + 1
	// - Server-side TUN IP = subnet IP + 2
	// - Client-side TUN gateway = subnet IP + 3
	// - Client-site TUN IP = subnet IP + 4

	sTUNIP := net.IPv4(subnetOctets[0], subnetOctets[1], subnetOctets[2], subnetOctets[3]+2)
	sTUNGateway := net.IPv4(subnetOctets[0], subnetOctets[1], subnetOctets[2], subnetOctets[3]+1)

	cTUNIP := net.IPv4(subnetOctets[0], subnetOctets[1], subnetOctets[2], subnetOctets[3]+4)
	cTUNGateway := net.IPv4(subnetOctets[0], subnetOctets[1], subnetOctets[2], subnetOctets[3]+3)

	if s.cfg.Secure {
		if err := BlockIPToLocalNetwork(cTUNIP, sTUNIP); err != nil {
			s.sendServerErrHello(conn, HandshakeStatusInternalError)
			return nil, nil, nil,
				fmt.Errorf("error securing local network for IP %s: %w", cTUNIP, err)
		}

		unsecureVPN = func() {
			if err := AllowIPToLocalNetwork(cTUNIP, sTUNIP); err != nil {
				s.log.WithError(err).Errorln("Error allowing traffic to local network")
			}
		}
	}

	sHello := ServerHello{
		Status:     HandshakeStatusOK,
		TUNIP:      cTUNIP,
		TUNGateway: cTUNGateway,
	}

	if err := WriteJSON(conn, &sHello); err != nil {
		unsecureVPN()
		return nil, nil, nil, fmt.Errorf("error finishing hadnshake: error sending server hello: %w", err)
	}

	return sTUNIP, sTUNGateway, unsecureVPN, nil
}

func (s *Server) sendServerErrHello(conn net.Conn, status HandshakeStatus) {
	sHello := ServerHello{
		Status: status,
	}

	if err := WriteJSON(conn, &sHello); err != nil {
		s.log.WithError(err).Errorln("Error sending server hello")
	}
}
