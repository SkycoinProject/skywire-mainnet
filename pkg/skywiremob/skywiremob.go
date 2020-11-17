package skywiremob

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/skycoin/skywire/pkg/visor/visorerr"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/internal/vpn"
	"github.com/skycoin/skywire/pkg/app/appnet"
	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/skyenv"
	"github.com/skycoin/skywire/pkg/visor"
	"github.com/skycoin/skywire/pkg/visor/visorconfig"
)

// here we redefine constants, so that this list would be exposed to the mobile app.
// We also have to case it to int, so the mobile app can work with these
const (
	ErrCodeNoError                = int(visorerr.ErrCodeNoError)
	ErrCodeInvalidPK              = int(visorerr.ErrCodeInvalidPK)
	ErrCodeInvalidVisorConfig     = int(visorerr.ErrCodeInvalidVisorConfig)
	ErrCodeInvalidAddrResolverURL = int(visorerr.ErrCodeInvalidAddrResolverURL)
	ErrCodeSTCPInitFailed         = int(visorerr.ErrCodeSTCPInitFailed)
	ErrCodeSTCPRInitFailed        = int(visorerr.ErrCodeSTCPRInitFailed)
	ErrCodeSUDPHInitFailed        = int(visorerr.ErrCodeSUDPHInitFailed)
	ErrCodeDmsgListenFailed       = int(visorerr.ErrCodeDmsgListenFailed)
	ErrCodeTpDiscUnavailable      = int(visorerr.ErrCodeTpDiscUnavailable)
	ErrCodeFailedToStartRouter    = int(visorerr.ErrCodeFailedToStartRouter)
	ErrCodeFailedToSetupHVGateway = int(visorerr.ErrCodeFailedToSetupHVGateway)

	ErrCodeUnknown = int(visorerr.ErrCodeUnknown)
)

const (
	ErrCodeVisorNotRunning int = iota + 901
	ErrCodeInvalidRemotePK
	ErrCodeFailedToSaveTransport
	ErrCodeVPNServerUnavailable
	ErrCodeVPNClientNotRunning
	ErrCodeHandshakeFailed
	ErrCodeInvalidAddr
	ErrCodeAlreadyListeningUDP
	ErrCodeUDPListenFailed
)

// Error is an error struct to be returned from `skywiremob`.
type Error struct {
	Code  int
	Error string
}

func newError(code int, err string) *Error {
	return &Error{
		Code:  code,
		Error: err,
	}
}

func newNoError() *Error {
	return &Error{
		Code: ErrCodeNoError,
	}
}

func errorFromVisorErrWithCode(e *visorerr.ErrorWithCode) *Error {
	err := Error{
		Code: int(e.Code),
	}

	if e.Code != visorerr.ErrCodeNoError && e.Err != nil {
		err.Error = e.Error()
	}

	return &err
}

const (
	visorConfig = `{
	"version": "v1.0.0",
	"sk": "${SK}",
	"pk": "${PK}",
	"dmsg": {
		"discovery": "http://dmsg.discovery.skywire.cc",
		"sessions_count": 1
	},
	"stcp": {
		"pk_table": null,
		"local_address": ":7777"
	},
	"transport": {
		"discovery": "http://transport.discovery.skywire.cc",
		"address_resolver": "http://address.resolver.skywire.cc",
		"log_store": {
			"type": "memory",
			"location": "./transport_logs"
		},
		"trusted_visors": null
	},
	"routing": {
		"setup_nodes": [
			"026c5a07de617c5c488195b76e8671bf9e7ee654d0633933e202af9e111ffa358d"
		],
		"route_finder": "http://routefinder.skywire.cc",
		"route_finder_timeout": "10s"
	},
	"uptime_tracker": {
		"addr": "http://uptime.tracker.skywire.cc"
	},
	"launcher": {
		"discovery": {
			"update_interval": "30s",
			"proxy_discovery_addr": "http://service.discovery.skywire.cc"
		},
		"apps": [
			{
				"name": "vpn-client",
				"auto_start": false,
				"port": 43
			}
		],
		"server_addr": "localhost:5505",
		"bin_path": "./apps",
		"local_path": "./local"
	},
	"hypervisors": [],
	"cli_addr": "localhost:3435",
	"log_level": "info",
	"shutdown_timeout": "10s",
	"restart_check_delay": "1s"
}`
)

// PrintString logs passed `str` with info log level.
func PrintString(str string) {
	log.Infoln(str)
}

// IsPKValid checks if pub key is valid. Returns non-empty error string on failure.
func IsPKValid(pkStr string) *Error {
	var pk cipher.PubKey
	if err := pk.UnmarshalText([]byte(pkStr)); err != nil {
		log.WithError(err).Errorln("Invalid PK")
		return newError(ErrCodeInvalidPK, err.Error())
	}

	return newNoError()
}

// GetMTU returns VPN connection MTU.
func GetMTU() int {
	return vpn.TUNMTU
}

// GetTUNIPPrefix returns netmask prefix of TUN IP address.
func GetTUNIPPrefix() int {
	return vpn.TUNNetmaskPrefix
}

var isVPNReady int32

// IsVPNReady checks if the VPN is ready.
func IsVPNReady() bool {
	return atomic.LoadInt32(&isVPNReady) == 1
}

var isVisorStarting int32

// IsVisorStarting checks if the visor is starting up.
func IsVisorStarting() bool {
	return atomic.LoadInt32(&isVisorStarting) == 1
}

var isVisorRunning int32

// IsVisorRunning checks if the visor is running.
func IsVisorRunning() bool {
	return atomic.LoadInt32(&isVisorRunning) == 1
}

var log = logging.NewMasterLogger()

var (
	globalVisorMx     sync.Mutex
	globalVisor       *visor.Visor
	globalVisorDone   = make(chan struct{})
	globalVisorReady  = make(chan error, 2)
	globalVisorCancel func()

	vpnClientMx sync.Mutex
	vpnClient   *vpn.ClientMobile

	tunCredsMx sync.Mutex
	tunIP      net.IP
	tunGateway net.IP
)

// PrepareVisor creates and runs visor instance.
func PrepareVisor() *Error {
	// set the same init stack as usual, but without apps launcher
	visor.SetInitStack(func() []visor.InitFunc {
		return []visor.InitFunc{
			visor.InitEventBroadcaster,
			visor.InitAddressResolver,
			visor.InitDiscovery,
			visor.InitSNet,
			visor.InitTransport,
			visor.InitRouter,
			visor.InitNetworkers,
			visor.InitHypervisors,
			visor.InitUptimeTracker,
			visor.InitTrustedVisors,
		}
	})

	// we use STDIN not to flush the config
	conf, err := initConfig(log, visorconfig.StdinName)
	if err != nil {
		return newError(ErrCodeInvalidVisorConfig, err.Error())
	}

	log.Infoln("Initialized config")

	v := visor.NewVisor(conf, nil)

	ctx, cancel := context.WithCancel(context.Background())

	globalVisorMx.Lock()
	globalVisor = v
	globalVisorCancel = cancel
	globalVisorMx.Unlock()

	go func() {
		<-ctx.Done()
		v.Close()
		globalVisorMx.Lock()
		close(globalVisorDone)
		globalVisorMx.Unlock()
	}()

	go func(ctx context.Context) {
		atomic.StoreInt32(&isVisorStarting, 1)

		var err error
		if err := v.Start(ctx); err != nil {
			log.WithError(err).Errorln("Failed to start visor")
		} else {
			atomic.StoreInt32(&isVisorStarting, 0)
			atomic.StoreInt32(&isVisorRunning, 1)
			log.Infoln("Started visor")
		}

		globalVisorMx.Lock()
		globalVisorReady <- err
		close(globalVisorReady)
		globalVisorMx.Unlock()
	}(ctx)

	return newNoError()
}

// WaitVisorReady blocks until visor gets fully initialized.
func WaitVisorReady() *Error {
	globalVisorMx.Lock()
	ready := globalVisorReady
	globalVisorMx.Unlock()

	if err := <-ready; err != nil {
		log.WithError(err).Errorln("Failed to start up visor")
		var e *visorerr.ErrorWithCode
		if errors.As(err, &e) {
			return errorFromVisorErrWithCode(e)
		}

		return newError(ErrCodeUnknown, err.Error())
	}

	return newNoError()
}

var (
	nextDmsgSocketIdxMx sync.Mutex
	nextDmsgSocketIdx   = -1
)

func getNextDmsgSocketIdx(ln int) int {
	nextDmsgSocketIdxMx.Lock()
	defer nextDmsgSocketIdxMx.Unlock()

	if nextDmsgSocketIdx == -2 {
		return -2
	}

	nextDmsgSocketIdx++
	if nextDmsgSocketIdx == ln {
		nextDmsgSocketIdx = -2
		return -2
	}

	return nextDmsgSocketIdx
}

// NextDmsgSocket returns next file descriptor of Dmsg socket. If no descriptors
// left or in case of error returns 0.
func NextDmsgSocket() int {
	globalVisorMx.Lock()
	v := globalVisor
	globalVisorMx.Unlock()

	if v == nil {
		return 0
	}

	allSessions := v.Network().Dmsg().AllSessions()
	log.Infof("Dmsg sockets count: %d\n", len(allSessions))

	nextDmsgSocketIdx := getNextDmsgSocketIdx(len(allSessions))
	if nextDmsgSocketIdx == -2 {
		return 0
	}

	conn := allSessions[nextDmsgSocketIdx].SessionCommon.GetConn()

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		log.Infoln("Failed to get Dmsg TCP conn")
		return 0
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		log.Infof("Failed to get Dmsg raw conn: %v\n", err)
		return 0
	}

	var fd uintptr
	var controlFunc = func(fdInner uintptr) {
		fd = fdInner
	}

	if err := rawConn.Control(controlFunc); err != nil {
		log.Infof("Failed to get Dmsg FD: %v\n", err)
		return 0
	}

	return int(fd)
}

// PrepareVPNClient creates and runs VPN client instance.
func PrepareVPNClient(srvPKStr, passcode string) *Error {
	globalVisorMx.Lock()
	v := globalVisor
	globalVisorMx.Unlock()

	if v == nil {
		return newError(ErrCodeVisorNotRunning, "Visor is not running")
	}

	var srvPK cipher.PubKey
	if err := srvPK.UnmarshalText([]byte(srvPKStr)); err != nil {
		return newError(ErrCodeInvalidRemotePK, err.Error())
	}

	if _, err := v.SaveTransport(context.Background(), srvPK, dmsg.Type); err != nil {
		return newError(ErrCodeFailedToSaveTransport, err.Error())
	}

	log.Infoln("Saved transport to VPN server")

	vpnPort := routing.Port(skyenv.VPNServerPort)

	connRaw, err := appnet.Dial(appnet.Addr{
		Net:    appnet.TypeSkynet,
		PubKey: srvPK,
		Port:   vpnPort,
	})
	if err != nil {
		return newError(ErrCodeVPNServerUnavailable, err.Error())
	}

	log.Infoln("Dialed VPN server")

	conn, err := appnet.WrapConn(connRaw)
	if err != nil {
		return newError(ErrCodeUnknown, fmt.Errorf("failed to wrap app conn: %w", err).Error())
	}

	log.Infoln("Wrapped app conn")

	vpnClientCfg := vpn.ClientConfig{
		Passcode: passcode,
	}

	vpnCl := vpn.NewClientMobile(vpnClientCfg, logrus.New(), conn)

	vpnClientMx.Lock()
	vpnClient = vpnCl
	vpnClientMx.Unlock()

	log.Infoln("Created VPN client")

	return newNoError()
}

// ShakeHands performs VPN client/server handshake.
func ShakeHands() *Error {
	vpnClientMx.Lock()
	vpnCl := vpnClient
	vpnClientMx.Unlock()

	if vpnCl == nil {
		return newError(ErrCodeVPNClientNotRunning, "VPN client is not running")
	}

	tunIPInternal, tunGatewayInternal, err := vpnCl.ShakeHands()
	if err != nil {
		return newError(ErrCodeHandshakeFailed, err.Error())
	}

	log.Infoln("Shook hands with VPN server")

	tunCredsMx.Lock()
	tunIP = tunIPInternal
	tunGateway = tunGatewayInternal
	tunCredsMx.Unlock()

	log.Println("Set TUN IP and gateway")

	return newNoError()
}

func getVPNSkywireConn() (*appnet.SkywireConn, bool) {
	vpnClientMx.Lock()
	vpnCl := vpnClient
	vpnClientMx.Unlock()

	if vpnCl == nil {
		return nil, false
	}

	wrappedConn := vpnCl.GetConn().(*appnet.WrappedConn)
	skywireConn := wrappedConn.Conn.(*appnet.SkywireConn)

	return skywireConn, true
}

// VPNBandwidthSent returns amount of bandwidth sent (bytes).
func VPNBandwidthSent() int64 {
	conn, ok := getVPNSkywireConn()
	if !ok {
		return 0
	}

	return int64(conn.BandwidthSent())
}

// VPNBandwidthReceived returns amount of bandwidth received (bytes).
func VPNBandwidthReceived() int64 {
	conn, ok := getVPNSkywireConn()
	if !ok {
		return 0
	}

	return int64(conn.BandwidthReceived())
}

// VPNLatency returns latency till remote (ms).
func VPNLatency() int64 {
	conn, ok := getVPNSkywireConn()
	if !ok {
		return 0
	}

	return conn.Latency().Milliseconds()
}

// VPNThroughput returns throughput till remote (bytes/s).
func VPNThroughput() int64 {
	conn, ok := getVPNSkywireConn()
	if !ok {
		return 0
	}

	return int64(conn.Throughput())
}

// TUNIP gets assigned TUN IP address.
func TUNIP() string {
	tunCredsMx.Lock()
	defer tunCredsMx.Unlock()

	if tunIP == nil {
		return ""
	}

	return tunIP.String()
}

// TUNGateway gets assigned TUN gateway.
func TUNGateway() string {
	tunCredsMx.Lock()
	defer tunCredsMx.Unlock()

	if tunGateway == nil {
		return ""
	}

	return tunGateway.String()
}

// StopVPNClient stops VPN client.
func StopVPNClient() {
	vpnClientMx.Lock()
	if vpnClient != nil {
		vpnClient.Close()
	}
	vpnClient = nil
	vpnClientMx.Unlock()

	atomic.StoreInt32(&isVPNReady, 0)

	close(mobileAppAddrCh)
	mobileAppAddrCh = make(chan *net.UDPAddr, 2)
}

// StopVisor stops running visor. Returns non-empty error string on failure.
func StopVisor() *Error {
	globalVisorMx.Lock()
	v := globalVisor
	cancel := globalVisorCancel
	vDone := globalVisorDone
	ready := globalVisorReady
	globalVisorMx.Unlock()

	if v == nil || cancel == nil {
		return newError(ErrCodeVisorNotRunning, "Visor is not running")
	}

	cancel()
	<-vDone
	<-ready

	atomic.StoreInt32(&isVisorStarting, 0)
	atomic.StoreInt32(&isVisorRunning, 0)

	StopVPNClient()

	StopListeningUDP()

	nextDmsgSocketIdxMx.Lock()
	nextDmsgSocketIdx = -1
	nextDmsgSocketIdxMx.Unlock()

	globalVisorMx.Lock()
	globalVisor = nil
	globalVisorCancel = nil
	globalVisorDone = make(chan struct{})
	globalVisorReady = make(chan error, 2)
	globalVisorMx.Unlock()

	return newNoError()
}

func initConfig(mLog *logging.MasterLogger, confPath string) (*visorconfig.V1, error) {
	pk, sk := cipher.GenerateKeyPair()

	parsedConf := strings.ReplaceAll(visorConfig, "${SK}", sk.String())
	parsedConf = strings.ReplaceAll(parsedConf, "${PK}", pk.String())
	conf, err := visorconfig.Parse(mLog, confPath, []byte(parsedConf))
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return conf, nil
}

var mobileAppAddrCh = make(chan *net.UDPAddr, 2)

// SetMobileAppAddr sets address of the UDP connection opened on the mobile application side.
func SetMobileAppAddr(addr string) *Error {
	// address passed from the Android device contains `/` prefix, strip it.
	addr = strings.TrimLeft(addr, " /")

	tokens := strings.Split(addr, ":")

	if len(tokens) != 2 {
		return newError(ErrCodeInvalidAddr,
			fmt.Errorf("android app addr is invalid (wrong tokens number): %s", addr).Error())
	}

	addrIP := net.ParseIP(tokens[0])
	addrPort, err := strconv.Atoi(tokens[1])
	if err != nil {
		return newError(ErrCodeInvalidAddr, err.Error())
	}

	mobileAppAddrCh <- &net.UDPAddr{
		IP:   addrIP,
		Port: addrPort,
		Zone: "",
	}

	return newNoError()
}

var (
	globalUDPConnMu sync.Mutex
	globalUDPConn   *net.UDPConn
)

// ServeVPN starts handling VPN traffic.
func ServeVPN() *Error {
	vpnClientMx.Lock()
	vpnCl := vpnClient
	vpnClientMx.Unlock()

	if vpnCl == nil {
		return newError(ErrCodeVPNClientNotRunning, "VPN client is not running")
	}

	go func() {
		tunAddr, ok := <-mobileAppAddrCh
		if !ok {
			log.Infoln("DIDN'T GET UDP ADDR, VISOR IS STOPPED, RETURNING...")
			return
		}

		log.Infof("Got mobile app UDP addr: %s\n", tunAddr.String())

		globalUDPConnMu.Lock()
		udpConn := globalUDPConn
		globalUDPConnMu.Unlock()

		if udpConn == nil {
			return
		}

		wr := vpn.NewUDPConnWriter(udpConn, tunAddr)

		if err := vpnCl.Serve(wr); err != nil {
			log.WithError(err).Errorln("Failed to serve VPN")
		}
	}()

	atomic.StoreInt32(&isVPNReady, 1)

	return newNoError()
}

// StartListeningUDP starts listening UDP.
func StartListeningUDP() *Error {
	globalUDPConnMu.Lock()
	if globalUDPConn != nil {
		globalUDPConnMu.Unlock()
		return newError(ErrCodeAlreadyListeningUDP, "UDP connection is already open")
	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IP{127, 0, 0, 1},
		Port: 7890,
	})
	if err != nil {
		return newError(ErrCodeUDPListenFailed, err.Error())
	}

	globalUDPConn = conn
	globalUDPConnMu.Unlock()

	log.Infoln("Listening UDP")

	return newNoError()
}

// StopListeningUDP closes UDP socket.
func StopListeningUDP() {
	globalUDPConnMu.Lock()
	if globalUDPConn != nil {
		if err := globalUDPConn.Close(); err != nil {
			log.WithError(err).Errorln("Failed to close mobile app UDP conn")
		}

		globalUDPConn = nil
	}
	globalUDPConnMu.Unlock()
}
