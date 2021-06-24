package vpn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/netutil"

	"github.com/skycoin/skywire/pkg/app"
	"github.com/skycoin/skywire/pkg/app/appnet"
	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/skyenv"
	skynetutil "github.com/skycoin/skywire/pkg/util/netutil"
)

const (
	ipv4FirstHalfAddr      = "0.0.0.0/1"
	ipv4SecondHalfAddr     = "128.0.0.0/1"
	directRouteNetmaskCIDR = "/32"
)

// Client is a VPN client.
type Client struct {
	log            *logrus.Logger
	cfg            ClientConfig
	appCl          *app.Client
	r              *netutil.Retrier
	directIPSMu    sync.Mutex
	directIPs      []net.IP
	defaultGateway net.IP
	closeC         chan struct{}
	closeOnce      sync.Once

	prevTUNGateway   net.IP
	prevTUNGatewayMu sync.Mutex

	suidMu sync.Mutex
	suid   int

	tunMu      sync.Mutex
	tun        TUNDevice
	tunCreated bool
}

// NewClient creates VPN client instance.
func NewClient(cfg ClientConfig, appCl *app.Client) (*Client, error) {
	dmsgDiscIP, err := dmsgDiscIPFromEnv()
	if err != nil {
		return nil, fmt.Errorf("error getting Dmsg discovery IP: %w", err)
	}

	dmsgSrvAddrs, err := dmsgSrvAddrsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("error getting Dmsg server addresses: %w", err)
	}

	tpDiscIP, err := tpDiscIPFromEnv()
	if err != nil {
		return nil, fmt.Errorf("error getting TP IP: %w", err)
	}

	arIP, err := addressResolverIPFromEnv()
	if err != nil {
		return nil, fmt.Errorf("error getting TP IP: %w", err)
	}

	rfIP, err := rfIPFromEnv()
	if err != nil {
		return nil, fmt.Errorf("error getting RF IP: %w", err)
	}

	utIP, err := uptimeTrackerIPFromEnv()
	if err != nil {
		return nil, fmt.Errorf("error getting UT IP: %w", err)
	}

	stcpEntities, err := stcpEntitiesFromEnv()
	if err != nil {
		return nil, fmt.Errorf("error getting STCP entities: %w", err)
	}

	tpRemoteIPs, err := tpRemoteIPsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("error getting TP remote IPs: %w", err)
	}

	requiredDirectIPs := []net.IP{dmsgDiscIP, tpDiscIP, rfIP}
	directIPs := make([]net.IP, 0, len(requiredDirectIPs)+len(dmsgSrvAddrs)+len(stcpEntities)+len(tpRemoteIPs))
	directIPs = append(directIPs, requiredDirectIPs...)
	directIPs = append(directIPs, dmsgSrvAddrs...)
	directIPs = append(directIPs, stcpEntities...)
	directIPs = append(directIPs, tpRemoteIPs...)

	if arIP != nil {
		directIPs = append(directIPs, arIP)
	}

	if utIP != nil {
		directIPs = append(directIPs, utIP)
	}

	const (
		serverDialInitBO = 1 * time.Second
		serverDialMaxBO  = 10 * time.Second
	)

	log := logrus.New()
	r := netutil.NewRetrier(log, serverDialInitBO, serverDialMaxBO, 0, 1)

	defaultGateway, err := DefaultNetworkGateway()
	if err != nil {
		return nil, fmt.Errorf("error getting default network gateway: %w", err)
	}

	fmt.Printf("Got default network gateway IP: %s\n", defaultGateway)

	return &Client{
		log:            log,
		cfg:            cfg,
		appCl:          appCl,
		r:              r,
		directIPs:      filterOutEqualIPs(directIPs),
		defaultGateway: defaultGateway,
		closeC:         make(chan struct{}),
	}, nil
}

// Serve dials VPN server, sets up TUN and establishes VPN session.
func (c *Client) Serve() error {
	if err := c.setSysPrivileges(); err != nil {
		return fmt.Errorf("failed to setup system privileges: %w", err)
	}
	// we setup direct routes to skywire services once for all the client lifetime since routes don't change.
	// but if they change, new routes get delivered to the app via callbacks.
	if err := c.setupDirectRoutes(); err != nil {
		c.releaseSysPrivileges()
		return fmt.Errorf("error setting up direct routes: %w", err)
	}
	c.releaseSysPrivileges()

	defer func() {
		if err := c.setSysPrivileges(); err != nil {
			fmt.Printf("failed to setup system privileges: %v\n", err)
			return
		}
		defer c.releaseSysPrivileges()

		c.removeDirectRoutes()
	}()

	// we call this preliminary, so it will be called on app stop
	defer func() {
		if c.cfg.Killswitch {
			err := c.setSysPrivileges()
			if err != nil {
				fmt.Printf("Error setting up system privileges: %v\n", err)
			} else {
				c.prevTUNGatewayMu.Lock()
				if len(c.prevTUNGateway) > 0 {
					fmt.Printf("Routing traffic directly, previous TUN gateway: %s\n", c.prevTUNGateway.String())
					c.routeTrafficDirectly(c.prevTUNGateway)
				}
				c.prevTUNGateway = nil
				c.prevTUNGatewayMu.Unlock()

				c.releaseSysPrivileges()
			}
		}

		if err := c.closeTUN(); err != nil {
			fmt.Printf("Failed to close TUN: %v\n", err)
		}

		fmt.Println("Closed TUN")
	}()

	defer c.setAppStatus(ClientStatusShuttingDown)

	c.setAppStatus(ClientStatusConnecting)

	r := netutil.NewDefaultRetrier(c.log)
	err := r.Do(context.Background(), func() error {
		if c.isClosed() {
			return nil
		}

		if err := c.dialServeConn(); err != nil {
			c.setAppStatus(ClientStatusReconnecting)
			fmt.Println("Connection broke, reconnecting...")
			return fmt.Errorf("dialServeConn: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to connect to the server: %w", err)
	}

	return nil
}

// Close closes client.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closeC)
	})
}

// AddDirectRoute adds new direct route. Packets destined to `ip` will
// go directly, ignoring VPN.
func (c *Client) AddDirectRoute(ip net.IP) error {
	c.directIPSMu.Lock()
	defer c.directIPSMu.Unlock()

	for _, storedIP := range c.directIPs {
		if ip.Equal(storedIP) {
			return nil
		}
	}

	c.directIPs = append(c.directIPs, ip)

	if err := c.setSysPrivileges(); err != nil {
		return fmt.Errorf("failed to setup system privileges: %w", err)
	}
	defer c.releaseSysPrivileges()

	return c.setupDirectRoute(ip)
}

// RemoveDirectRoute removes direct route. Packets destined to `ip` will
// go through VPN.
func (c *Client) RemoveDirectRoute(ip net.IP) error {
	c.directIPSMu.Lock()
	defer c.directIPSMu.Unlock()

	for i, storedIP := range c.directIPs {
		if ip.Equal(storedIP) {
			c.directIPs = append(c.directIPs[:i], c.directIPs[i+1:]...)

			if err := c.setSysPrivileges(); err != nil {
				return fmt.Errorf("failed to setup system privileges: %w", err)
			}
			defer c.releaseSysPrivileges()

			if err := c.removeDirectRoute(ip); err != nil {
				return err
			}

			break
		}
	}

	return nil
}

func (c *Client) setSysPrivileges() error {
	c.suidMu.Lock()

	// we don't release the lock here to avoid races,
	// lock will be released after reverting system privileges

	suid, err := setupClientSysPrivileges()
	if err != nil {
		return err
	}

	c.suid = suid

	return nil
}

func (c *Client) createTUN() (TUNDevice, error) {
	c.tunMu.Lock()
	defer c.tunMu.Unlock()

	if c.tunCreated {
		return c.tun, nil
	}

	tun, err := newTUNDevice()
	if err != nil {
		return nil, err
	}

	c.tun = tun
	c.tunCreated = true

	return tun, nil
}

func (c *Client) closeTUN() error {
	c.tunMu.Lock()
	defer c.tunMu.Unlock()

	if !c.tunCreated {
		return nil
	}

	c.tunCreated = false

	return c.tun.Close()
}

func (c *Client) setupTUN(tunIP, tunGateway net.IP) error {
	c.tunMu.Lock()
	defer c.tunMu.Unlock()

	if !c.tunCreated {
		return errors.New("TUN is not created")
	}

	return SetupTUN(c.tun.Name(), tunIP.String()+TUNNetmaskCIDR, tunGateway.String(), TUNMTU)
}

func (c *Client) serveConn(conn net.Conn) error {
	tunIP, tunGateway, err := c.shakeHands(conn)
	if err != nil {
		return fmt.Errorf("error during client/server handshake: %w", err)
	}

	fmt.Printf("Performed handshake with %s\n", conn.RemoteAddr())
	fmt.Printf("Local TUN IP: %s\n", tunIP.String())
	fmt.Printf("Local TUN gateway: %s\n", tunGateway.String())

	if err := c.setSysPrivileges(); err != nil {
		return fmt.Errorf("failed to setup system privileges: %w", err)
	}

	// this call is important. it will either run on an error down the line,
	// or, in case VPN sessions finishes, it will be the last call in deferred stack,
	// releasing system privileges after cleanup
	defer c.releaseSysPrivileges()

	tun, err := c.createTUN()
	if err != nil {
		return fmt.Errorf("error allocating TUN interface: %w", err)
	}

	// we don't defer TUN closing routine here, so that the interface might be
	// reused. this function may end in 2 cases: connection failure, app stop.
	// in case app got stopped, TUN will be closed in the outer code, while in
	// case of connection failure it will be reused

	fmt.Printf("Allocated TUN %s: %v\n", tun.Name(), err)

	if err := c.setupTUN(tunIP, tunGateway); err != nil {
		return fmt.Errorf("error setting up TUN %s: %w", tun.Name(), err)
	}

	if runtime.GOOS == "windows" {
		// okay, so, here's done because after the `SetupTUN` call,
		// interface doesn't get its values immediately. Reason is unknown,
		// all credits go to Microsoft. Delay may be different, this one is
		// fairly large to cover not really performant systems.
		time.Sleep(10 * time.Second)
	}

	isNewRoute := true
	if c.cfg.Killswitch {
		c.prevTUNGatewayMu.Lock()
		if len(c.prevTUNGateway) > 0 {
			isNewRoute = false
		}
		c.prevTUNGateway = tunGateway
		c.prevTUNGatewayMu.Unlock()
	}

	fmt.Printf("Routing all traffic through TUN %s: %v\n", tun.Name(), err)
	if err := c.routeTrafficThroughTUN(tunGateway, isNewRoute); err != nil {
		return fmt.Errorf("error routing traffic through TUN %s: %w", tun.Name(), err)
	}

	c.setAppStatus(ClientStatusRunning)

	defer func() {
		if !c.cfg.Killswitch {
			fmt.Println("serveConn done, killswitch disabled, routing traffic directly")
			c.routeTrafficDirectly(tunGateway)
		}
	}()

	// we release privileges here (user is not root for Mac OS systems from here on)
	c.releaseSysPrivileges()

	connToTunDoneCh := make(chan struct{})
	tunToConnCh := make(chan struct{})
	// read all system traffic and pass it to the remote VPN server
	go func() {
		defer close(connToTunDoneCh)

		if _, err := io.Copy(tun, conn); err != nil {
			fmt.Printf("Error resending traffic from TUN %s to VPN server: %v\n", tun.Name(), err)
		}
	}()
	go func() {
		defer close(tunToConnCh)

		if _, err := io.Copy(conn, tun); err != nil {
			fmt.Printf("Error resending traffic from VPN server to TUN %s: %v\n", tun.Name(), err)
		}
	}()

	// only one side may fail here, so we wait till at least one fails
	select {
	case <-connToTunDoneCh:
	case <-tunToConnCh:
	case <-c.closeC:
	}

	// here we setup system privileges again, so deferred calls may be done safely
	if err := c.setSysPrivileges(); err != nil {
		fmt.Printf("Failed to setup system privileges for cleanup: %v\n", err)
	}

	return nil
}

func (c *Client) dialServeConn() error {
	conn, err := c.dialServer(c.appCl, c.cfg.ServerPK)
	if err != nil {
		return fmt.Errorf("error connecting to VPN server: %w", err)
	}

	fmt.Printf("Dialed %s\n", conn.RemoteAddr())

	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Printf("Error closing app conn: %v\n", err)
		}
	}()

	if c.isClosed() {
		return nil
	}

	if err := c.serveConn(conn); err != nil {
		return fmt.Errorf("error serving app conn: %w", err)
	}

	return nil
}

func (c *Client) routeTrafficThroughTUN(tunGateway net.IP, isNewRoute bool) error {
	// route all traffic through TUN gateway
	if isNewRoute {
		if err := AddRoute(ipv4FirstHalfAddr, tunGateway.String()); err != nil {
			return err
		}
		if err := AddRoute(ipv4SecondHalfAddr, tunGateway.String()); err != nil {
			return err
		}
	} else {
		if err := ChangeRoute(ipv4FirstHalfAddr, tunGateway.String()); err != nil {
			return err
		}
		if err := ChangeRoute(ipv4SecondHalfAddr, tunGateway.String()); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) routeTrafficDirectly(tunGateway net.IP) {
	fmt.Println("Routing all traffic through default network gateway")

	// remove main route
	if err := DeleteRoute(ipv4FirstHalfAddr, tunGateway.String()); err != nil {
		fmt.Printf("Error routing traffic through default network gateway: %v\n", err)
	}
	if err := DeleteRoute(ipv4SecondHalfAddr, tunGateway.String()); err != nil {
		fmt.Printf("Error routing traffic through default network gateway: %v\n", err)
	}
}

func (c *Client) setupDirectRoutes() error {
	c.directIPSMu.Lock()
	defer c.directIPSMu.Unlock()

	for _, ip := range c.directIPs {
		if err := c.setupDirectRoute(ip); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) setupDirectRoute(ip net.IP) error {
	if !ip.IsLoopback() {
		fmt.Printf("Adding direct route to %s, via %s", ip.String(), c.defaultGateway.String())
		if err := AddRoute(ip.String()+directRouteNetmaskCIDR, c.defaultGateway.String()); err != nil {
			return fmt.Errorf("error adding direct route to %s: %w", ip.String(), err)
		}
	}

	return nil
}

func (c *Client) removeDirectRoute(ip net.IP) error {
	if !ip.IsLoopback() {
		fmt.Printf("Removing direct route to %s\n", ip.String())
		if err := DeleteRoute(ip.String()+directRouteNetmaskCIDR, c.defaultGateway.String()); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) removeDirectRoutes() {
	c.directIPSMu.Lock()
	defer c.directIPSMu.Unlock()

	for _, ip := range c.directIPs {
		if err := c.removeDirectRoute(ip); err != nil {
			// shouldn't return, just keep on trying the other IPs
			fmt.Printf("Error removing direct route to %s: %v\n", ip.String(), err)
		}
	}
}

func dmsgDiscIPFromEnv() (net.IP, error) {
	return ipFromEnv(DmsgDiscAddrEnvKey)
}

func dmsgSrvAddrsFromEnv() ([]net.IP, error) {
	dmsgSrvCountStr := os.Getenv(DmsgAddrsCountEnvKey)
	if dmsgSrvCountStr == "" {
		return nil, errors.New("dmsg servers count is not provided")
	}
	dmsgSrvCount, err := strconv.Atoi(dmsgSrvCountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid Dmsg servers count: %s: %w", dmsgSrvCountStr, err)
	}

	dmsgSrvAddrs := make([]net.IP, 0, dmsgSrvCount)
	for i := 0; i < dmsgSrvCount; i++ {
		dmsgSrvAddr, err := ipFromEnv(DmsgAddrEnvPrefix + strconv.Itoa(i))
		if err != nil {
			return nil, fmt.Errorf("error getting Dmsg server address: %w", err)
		}

		dmsgSrvAddrs = append(dmsgSrvAddrs, dmsgSrvAddr)
	}

	return dmsgSrvAddrs, nil
}

func tpDiscIPFromEnv() (net.IP, error) {
	return ipFromEnv(TPDiscAddrEnvKey)
}

func addressResolverIPFromEnv() (net.IP, error) {
	return ipFromEnv(AddressResolverAddrEnvKey)
}

func rfIPFromEnv() (net.IP, error) {
	return ipFromEnv(RFAddrEnvKey)
}

func uptimeTrackerIPFromEnv() (net.IP, error) {
	return ipFromEnv(UptimeTrackerAddrEnvKey)
}

func tpRemoteIPsFromEnv() ([]net.IP, error) {
	var ips []net.IP
	ipsLenStr := os.Getenv(TPRemoteIPsLenEnvKey)
	if ipsLenStr == "" {
		return nil, nil
	}

	ipsLen, err := strconv.Atoi(ipsLenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid TPs remote IPs len: %s: %w", ipsLenStr, err)
	}

	ips = make([]net.IP, 0, ipsLen)
	for i := 0; i < ipsLen; i++ {
		key := TPRemoteIPsEnvPrefix + strconv.Itoa(i)

		ipStr := os.Getenv(key)
		if ipStr == "" {
			return nil, fmt.Errorf("env arg %s is not provided", key)
		}

		ip, err := ipFromEnv(key)
		if err != nil {
			return nil, fmt.Errorf("error getting TP remote IP: %w", err)
		}

		ips = append(ips, ip)
	}

	return ips, nil
}

func stcpEntitiesFromEnv() ([]net.IP, error) {
	var stcpEntities []net.IP
	stcpTableLenStr := os.Getenv(STCPTableLenEnvKey)
	if stcpTableLenStr != "" {
		stcpTableLen, err := strconv.Atoi(stcpTableLenStr)
		if err != nil {
			return nil, fmt.Errorf("invalid STCP table len: %s: %w", stcpTableLenStr, err)
		}

		stcpEntities = make([]net.IP, 0, stcpTableLen)
		for i := 0; i < stcpTableLen; i++ {
			stcpKey := os.Getenv(STCPKeyEnvPrefix + strconv.Itoa(i))
			if stcpKey == "" {
				return nil, fmt.Errorf("env arg %s is not provided", STCPKeyEnvPrefix+strconv.Itoa(i))
			}

			stcpAddr, err := ipFromEnv(STCPValueEnvPrefix + stcpKey)
			if err != nil {
				return nil, fmt.Errorf("error getting STCP entity IP: %w", err)
			}

			stcpEntities = append(stcpEntities, stcpAddr)
		}
	}

	return stcpEntities, nil
}

func (c *Client) shakeHands(conn net.Conn) (TUNIP, TUNGateway net.IP, err error) {
	unavailableIPs, err := skynetutil.LocalNetworkInterfaceIPs()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting unavailable private IPs: %w", err)
	}

	unavailableIPs = append(unavailableIPs, c.defaultGateway)

	cHello := ClientHello{
		UnavailablePrivateIPs: unavailableIPs,
		Passcode:              c.cfg.Passcode,
	}

	const handshakeTimeout = 5 * time.Second

	fmt.Printf("Sending client hello: %v\n", cHello)

	if err := WriteJSONWithTimeout(conn, &cHello, handshakeTimeout); err != nil {
		return nil, nil, fmt.Errorf("error sending client hello: %w", err)
	}

	var sHello ServerHello
	if err := ReadJSONWithTimeout(conn, &sHello, handshakeTimeout); err != nil {
		return nil, nil, fmt.Errorf("error reading server hello: %w", err)
	}

	fmt.Printf("Got server hello: %v", sHello)

	if sHello.Status != HandshakeStatusOK {
		return nil, nil, fmt.Errorf("got status %d (%s) from the server", sHello.Status, sHello.Status)
	}

	return sHello.TUNIP, sHello.TUNGateway, nil
}

func (c *Client) dialServer(appCl *app.Client, pk cipher.PubKey) (net.Conn, error) {
	const (
		netType = appnet.TypeSkynet
		vpnPort = routing.Port(skyenv.VPNServerPort)
	)

	var conn net.Conn
	err := c.r.Do(context.Background(), func() error {
		var err error
		conn, err = appCl.Dial(appnet.Addr{
			Net:    netType,
			PubKey: pk,
			Port:   vpnPort,
		})

		if c.isClosed() {
			// in this case client got closed, we return no error,
			// so that retrier could stop gracefully
			return nil
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	if c.isClosed() {
		// we need to signal outer code that connection object is inalid
		// in this case
		return nil, errors.New("client got closed")
	}

	return conn, nil
}

func (c *Client) setAppStatus(status ClientStatus) {
	if err := c.appCl.SetDetailedStatus(string(status)); err != nil {
		fmt.Printf("Failed to set status %v: %v\n", status, err)
	}
}

func (c *Client) isClosed() bool {
	select {
	case <-c.closeC:
		return true
	default:
	}

	return false
}

func ipFromEnv(key string) (net.IP, error) {
	ip, ok, err := IPFromEnv(key)
	if err != nil {
		return nil, fmt.Errorf("error getting IP from %s: %w", key, err)
	}
	if !ok {
		return nil, fmt.Errorf("env arg %s is not provided", key)
	}

	return ip, nil
}

func filterOutEqualIPs(ips []net.IP) []net.IP {
	ipsSet := make(map[string]struct{})
	var filteredIPs []net.IP
	for _, ip := range ips {
		ipStr := ip.String()

		if _, ok := ipsSet[ipStr]; !ok {
			filteredIPs = append(filteredIPs, ip)
			ipsSet[ip.String()] = struct{}{}
		}
	}

	return filteredIPs
}
