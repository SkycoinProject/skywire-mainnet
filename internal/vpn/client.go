package vpn

import (
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
)

const (
	ipv4FirstHalfAddr      = "0.0.0.0/1"
	ipv4SecondHalfAddr     = "128.0.0.0/1"
	directRouteNetmaskCIDR = "/32"
)

// Client is a VPN client.
type Client struct {
	cfg            ClientConfig
	log            logrus.FieldLogger
	conn           net.Conn
	directIPSMu    sync.Mutex
	directIPs      []net.IP
	defaultGateway net.IP
	closeC         chan struct{}
	closeOnce      sync.Once
}

// NewClient creates VPN client instance.
func NewClient(cfg ClientConfig, l logrus.FieldLogger, conn net.Conn) (*Client, error) {
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

	defaultGateway, err := DefaultNetworkGateway()
	if err != nil {
		return nil, fmt.Errorf("error getting default network gateway: %w", err)
	}

	fmt.Printf("Got default network gateway IP: %s\n", defaultGateway)

	return &Client{
		cfg:            cfg,
		log:            l,
		conn:           conn,
		directIPs:      filterOutEqualIPs(directIPs),
		defaultGateway: defaultGateway,
		closeC:         make(chan struct{}),
	}, nil
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

	suid, err := setupClientSysPrivileges()
	if err != nil {
		return fmt.Errorf("failed to setup system privileges: %w", err)
	}

	defer c.releaseSysPrivileges(suid)

	if err := c.setupDirectRoute(ip); err != nil {
		return err
	}

	return nil
}

// RemoveDirectRoute removes direct route. Packets destined to `ip` will
// go through VPN.
func (c *Client) RemoveDirectRoute(ip net.IP) error {
	c.directIPSMu.Lock()
	defer c.directIPSMu.Unlock()

	for i, storedIP := range c.directIPs {
		if ip.Equal(storedIP) {
			c.directIPs = append(c.directIPs[:i], c.directIPs[i+1:]...)

			if err := c.removeDirectRoute(ip); err != nil {
				return err
			}
		}
	}

	return nil
}

// Serve performs handshake with the server, sets up routing and starts handling traffic.
func (c *Client) Serve() error {
	tunIP, tunGateway, err := c.shakeHands()
	if err != nil {
		return fmt.Errorf("error during client/server handshake: %w", err)
	}

	fmt.Printf("Performed handshake with %s\n", c.conn.RemoteAddr())
	fmt.Printf("Local TUN IP: %s\n", tunIP.String())
	fmt.Printf("Local TUN gateway: %s\n", tunGateway.String())

	suid, err := setupClientSysPrivileges()
	if err != nil {
		return fmt.Errorf("failed to setup system privileges: %w", err)
	}

	tun, err := newTUNDevice()
	if err != nil {
		c.releaseSysPrivileges(suid)
		return fmt.Errorf("error allocating TUN interface: %w", err)
	}
	defer func() {
		tunName := tun.Name()
		if err := tun.Close(); err != nil {
			fmt.Printf("Error closing TUN %s: %v\n", tunName, err)
		}

		c.releaseSysPrivileges(suid)
	}()

	fmt.Printf("Allocated TUN %s: %v\n", tun.Name(), err)

	err = SetupTUN(tun.Name(), tunIP.String()+"/"+strconv.Itoa(TUNNetmaskPrefix), tunGateway.String(), TUNMTU)
	if err != nil {
		return fmt.Errorf("error setting up TUN %s: %w", tun.Name(), err)
	}

	defer c.removeDirectRoutes()
	if err := c.setupDirectRoutes(); err != nil {
		return fmt.Errorf("error setting up direct routes: %w", err)
	}

	if runtime.GOOS == "windows" {
		// okay, so, here's done because after the `SetupTUN` call,
		// interface doesn't get its values immediately. Reason is unknown,
		// all credits go to Microsoft. Delay may be different, this one is
		// fairly large to cover not really performant systems.
		time.Sleep(10 * time.Second)
	}

	defer c.routeTrafficDirectly(tunGateway)
	fmt.Printf("Routing all traffic through TUN %s: %v\n", tun.Name(), err)
	if err := c.routeTrafficThroughTUN(tunGateway); err != nil {
		return fmt.Errorf("error routing traffic through TUN %s: %w", tun.Name(), err)
	}

	// we release privileges here (user is not root for Mac OS systems from here on)
	c.releaseSysPrivileges(suid)
	// this will be executed first on return, so we setup privileges once again,
	// so other deferred clear up calls may be done successfully
	if _, err := setupClientSysPrivileges(); err != nil {
		fmt.Printf("Failed to setup system privileges to clear up: %v\n", err)
	}

	connToTunDoneCh := make(chan struct{})
	tunToConnCh := make(chan struct{})
	// read all system traffic and pass it to the remote VPN server
	go func() {
		defer close(connToTunDoneCh)

		if _, err := io.Copy(tun, c.conn); err != nil {
			fmt.Printf("Error resending traffic from TUN %s to VPN server: %v\n", tun.Name(), err)
		}
	}()
	go func() {
		defer close(tunToConnCh)

		if _, err := io.Copy(c.conn, tun); err != nil {
			fmt.Printf("Error resending traffic from VPN server to TUN %s: %v\n", tun.Name(), err)
		}
	}()

	// only one side may fail here, so we wait till at least one fails
	select {
	case <-connToTunDoneCh:
	case <-tunToConnCh:
	case <-c.closeC:
	}

	return nil
}

// Close closes client.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closeC)
	})
}

func (c *Client) routeTrafficThroughTUN(tunGateway net.IP) error {
	// route all traffic through TUN gateway
	if err := AddRoute(ipv4FirstHalfAddr, tunGateway.String()); err != nil {
		return err
	}
	if err := AddRoute(ipv4SecondHalfAddr, tunGateway.String()); err != nil {
		return err
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

func (c *Client) shakeHands() (TUNIP, TUNGateway net.IP, err error) {
	unavailableIPs, err := LocalNetworkInterfaceIPs()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting unavailable private IPs: %w", err)
	}

	unavailableIPs = append(unavailableIPs, c.defaultGateway)

	cHello := ClientHello{
		UnavailablePrivateIPs: unavailableIPs,
		Passcode:              c.cfg.Passcode,
	}

	return DoClientHandshake(c.log, c.conn, cHello)
}

func (c *Client) releaseSysPrivileges(suid int) {
	if err := releaseClientSysPrivileges(suid); err != nil {
		fmt.Printf("Failed to release system privileges: %v\n", err)
	}
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
