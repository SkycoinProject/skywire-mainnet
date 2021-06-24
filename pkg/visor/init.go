package visor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/dmsgctrl"
	"github.com/skycoin/dmsg/dmsgpty"
	dmsgnetutil "github.com/skycoin/dmsg/netutil"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/internal/utclient"
	"github.com/skycoin/skywire/internal/vpn"
	"github.com/skycoin/skywire/pkg/app/appdisc"
	"github.com/skycoin/skywire/pkg/app/appevent"
	"github.com/skycoin/skywire/pkg/app/appserver"
	"github.com/skycoin/skywire/pkg/app/launcher"
	"github.com/skycoin/skywire/pkg/routefinder/rfclient"
	"github.com/skycoin/skywire/pkg/router"
	"github.com/skycoin/skywire/pkg/servicedisc"
	"github.com/skycoin/skywire/pkg/setup/setupclient"
	"github.com/skycoin/skywire/pkg/skyenv"
	"github.com/skycoin/skywire/pkg/snet"
	"github.com/skycoin/skywire/pkg/snet/arclient"
	"github.com/skycoin/skywire/pkg/snet/directtp/tptypes"
	"github.com/skycoin/skywire/pkg/transport"
	ts "github.com/skycoin/skywire/pkg/transport/setup"
	"github.com/skycoin/skywire/pkg/transport/tpdclient"
	"github.com/skycoin/skywire/pkg/util/netutil"
	"github.com/skycoin/skywire/pkg/util/osutil"
	"github.com/skycoin/skywire/pkg/util/updater"
	"github.com/skycoin/skywire/pkg/visor/visorconfig"
	vinit "github.com/skycoin/skywire/pkg/visor/visorinit"
)

type visorCtxKey int

const visorKey visorCtxKey = iota

type runtimeErrsCtxKey int

const runtimeErrsKey runtimeErrsCtxKey = iota

// Visor initialization is split into modules, that can be initialized independently
// Modules are declared here as package-level variables, but also need to be registered
// in the modules system: they need init function and dependencies and their name to be set
// To add new piece of functionality to visor, you need to create a new module variable
// and register it properly in registerModules function
var (
	// Event broadcasting system
	ebc vinit.Module
	// visor updater
	up vinit.Module
	// Address resolver
	ar vinit.Module
	// App discovery
	disc vinit.Module
	// Snet (different network types)
	sn vinit.Module
	// dmsg pty: a remote terminal to the visor working over dmsg protocol
	pty vinit.Module
	// Transport manager
	tr vinit.Module
	// Transport setup
	trs vinit.Module
	// Routing system
	rt vinit.Module
	// Application launcer
	launch vinit.Module
	// CLI
	cli vinit.Module
	// hypervisors to control this visor
	hvs vinit.Module
	// Uptime tracker
	ut vinit.Module
	// Public visors: automatically establish connections to public visors
	pvs vinit.Module
	// Public visor: advertise current visor as public
	pv vinit.Module
	// hypervisor module
	hv vinit.Module
	// dmsg ctrl
	dmsgCtrl vinit.Module
	// visor that groups all modules together
	vis vinit.Module
)

// register all modules: instantiate modules with correct names and dependencies, wrap init
// functions to have access to visor and runtime errors channel
func registerModules(logger *logging.MasterLogger) {
	// utility module maker, to avoid passing logger and wrapping each init function
	// in withVisorCtx
	maker := func(name string, f initFn, deps ...*vinit.Module) vinit.Module {
		return vinit.MakeModule(name, withInitCtx(f), logger, deps...)
	}
	up = maker("updater", initUpdater)
	ebc = maker("event_broadcaster", initEventBroadcaster)
	ar = maker("address_resolver", initAddressResolver)
	disc = maker("discovery", initDiscovery)
	sn = maker("snet", initSNet, &ar, &disc, &ebc)
	dmsgCtrl = maker("dmsg_ctrl", initDmsgCtrl, &sn)
	pty = maker("dmsg_pty", initDmsgpty, &sn)
	tr = maker("transport", initTransport, &sn, &ebc)
	trs = maker("transport_setup", initTransportSetup, &sn, &tr)
	rt = maker("router", initRouter, &tr, &sn)
	launch = maker("launcher", initLauncher, &ebc, &disc, &sn, &tr, &rt)
	cli = maker("cli", initCLI)
	hvs = maker("hypervisors", initHypervisors, &sn)
	ut = maker("uptime_tracker", initUptimeTracker)
	pv = maker("public_visors", initPublicVisors, &tr)
	pvs = maker("public_visor", initPublicVisor, &sn, &ar, &disc)
	vis = vinit.MakeModule("visor", vinit.DoNothing, logger, &up, &ebc, &ar, &disc, &sn, &pty,
		&tr, &rt, &launch, &cli, &hvs, &ut, &pv, &pvs, &trs, &dmsgCtrl)

	hv = maker("hypervisor", initHypervisor, &vis)
}

type initFn func(context.Context, *Visor, *logging.Logger) error

func initUpdater(ctx context.Context, v *Visor, log *logging.Logger) error {
	updater := updater.New(v.log, v.restartCtx, v.conf.Launcher.BinPath)

	v.initLock.Lock()
	defer v.initLock.Unlock()
	v.restartCtx.SetCheckDelay(time.Duration(v.conf.RestartCheckDelay))
	v.restartCtx.RegisterLogger(v.log)
	v.updater = updater
	return nil
}

func initEventBroadcaster(ctx context.Context, v *Visor, log *logging.Logger) error {
	const ebcTimeout = time.Second
	ebc := appevent.NewBroadcaster(log, ebcTimeout)
	v.pushCloseStack("event_broadcaster", ebc.Close)

	v.initLock.Lock()
	v.ebc = ebc
	v.initLock.Unlock()
	return nil
}

func initAddressResolver(ctx context.Context, v *Visor, log *logging.Logger) error {
	conf := v.conf.Transport

	arClient, err := arclient.NewHTTP(conf.AddressResolver, v.conf.PK, v.conf.SK, log)
	if err != nil {
		err := fmt.Errorf("failed to create address resolver client: %w", err)
		return err
	}
	v.initLock.Lock()
	v.arClient = arClient
	v.initLock.Unlock()
	return nil
}

func initDiscovery(ctx context.Context, v *Visor, log *logging.Logger) error {
	// Prepare app discovery factory.
	factory := appdisc.Factory{
		Log: v.MasterLogger().PackageLogger("app_discovery"),
	}

	conf := v.conf.Launcher

	if conf.Discovery != nil {
		factory.PK = v.conf.PK
		factory.SK = v.conf.SK
		factory.UpdateInterval = time.Duration(conf.Discovery.UpdateInterval)
		factory.ProxyDisc = conf.Discovery.ServiceDisc
	}
	v.initLock.Lock()
	v.serviceDisc = factory
	v.initLock.Unlock()
	return nil
}

func initSNet(ctx context.Context, v *Visor, log *logging.Logger) error {
	nc := snet.NetworkConfigs{
		Dmsg: v.conf.Dmsg,
		STCP: v.conf.STCP,
	}

	conf := snet.Config{
		PubKey:         v.conf.PK,
		SecKey:         v.conf.SK,
		ARClient:       v.arClient,
		NetworkConfigs: nc,
	}

	n, err := snet.New(conf, v.ebc, v.MasterLogger())
	if err != nil {
		return err
	}

	if err := n.Init(); err != nil {
		return err
	}
	v.pushCloseStack("snet", n.Close)

	v.initLock.Lock()
	v.net = n
	v.initLock.Unlock()
	return nil
}

func initDmsgCtrl(ctx context.Context, v *Visor, _ *logging.Logger) error {
	dmsgC := v.net.Dmsg()
	if dmsgC == nil {
		return nil
	}
	const dmsgTimeout = time.Second * 20
	logger := dmsgC.Logger().WithField("timeout", dmsgTimeout)
	logger.Info("Connecting to the dmsg network...")
	select {
	case <-time.After(dmsgTimeout):
		logger.Warn("Failed to connect to the dmsg network, will try again later.")
	case <-v.net.Dmsg().Ready():
		logger.Info("Connected to the dmsg network.")
	}
	// dmsgctrl setup
	cl, err := dmsgC.Listen(skyenv.DmsgCtrlPort)
	if err != nil {
		return err
	}
	v.pushCloseStack("snet.dmsgctrl", cl.Close)

	dmsgctrl.ServeListener(cl, 0)
	return nil
}

const ownerRWX = 0700

func initDmsgpty(ctx context.Context, v *Visor, log *logging.Logger) error {
	conf := v.conf.Dmsgpty

	if conf == nil {
		log.Info("'dmsgpty' is not configured, skipping.")
		return nil
	}

	// Unlink dmsg socket files (just in case).
	if conf.CLINet == "unix" {
		if err := osutil.UnlinkSocketFiles(v.conf.Dmsgpty.CLIAddr); err != nil {
			return err
		}
	}

	wl := dmsgpty.NewMemoryWhitelist()

	// Ensure hypervisors are added to the whitelist.
	if err := wl.Add(v.conf.Hypervisors...); err != nil {
		return err
	}
	// add itself to the whitelist to allow local pty
	if err := wl.Add(v.conf.PK); err != nil {
		v.log.Errorf("Cannot add itself to the pty whitelist: %s", err)
	}

	dmsgC := v.net.Dmsg()
	if dmsgC == nil {
		err := errors.New("cannot create dmsgpty with nil dmsg client")
		return err
	}

	pty := dmsgpty.NewHost(dmsgC, wl)

	if ptyPort := conf.Port; ptyPort != 0 {
		serveCtx, cancel := context.WithCancel(context.Background())
		wg := new(sync.WaitGroup)
		wg.Add(1)

		go func() {
			defer wg.Done()
			runtimeErrors := getErrors(ctx)
			if err := pty.ListenAndServe(serveCtx, ptyPort); err != nil {
				runtimeErrors <- fmt.Errorf("listen and serve stopped: %w", err)
			}
		}()

		v.pushCloseStack("router.serve", func() error {
			cancel()
			wg.Wait()
			return nil
		})

	}

	if conf.CLINet != "" {
		if conf.CLINet == "unix" {
			if err := os.MkdirAll(filepath.Dir(conf.CLIAddr), ownerRWX); err != nil {
				err := fmt.Errorf("failed to prepare unix file for dmsgpty cli listener: %w", err)
				return err
			}
		}

		cliL, err := net.Listen(conf.CLINet, conf.CLIAddr)
		if err != nil {
			err := fmt.Errorf("failed to start dmsgpty cli listener: %w", err)
			return err
		}

		serveCtx, cancel := context.WithCancel(context.Background())
		wg := new(sync.WaitGroup)
		wg.Add(1)

		go func() {
			defer wg.Done()
			runtimeErrors := getErrors(ctx)
			if err := pty.ServeCLI(serveCtx, cliL); err != nil {
				runtimeErrors <- fmt.Errorf("serve cli stopped: %w", err)
			}
		}()

		v.pushCloseStack("router.serve", func() error {
			cancel()
			err := cliL.Close()
			wg.Wait()
			return err
		})
	}

	return nil
}

func initTransport(ctx context.Context, v *Visor, log *logging.Logger) error {

	tpdC, err := connectToTpDisc(v)
	if err != nil {
		err := fmt.Errorf("failed to create transport discovery client: %w", err)
		return err
	}

	logS := transport.InMemoryTransportLogStore()

	tpMConf := transport.ManagerConfig{
		PubKey:          v.conf.PK,
		SecKey:          v.conf.SK,
		DiscoveryClient: tpdC,
		LogStore:        logS,
	}
	managerLogger := v.MasterLogger().PackageLogger("transport_manager")
	tpM, err := transport.NewManager(managerLogger, v.net, &tpMConf)
	if err != nil {
		err := fmt.Errorf("failed to start transport manager: %w", err)
		return err
	}

	tpM.OnAfterTPClosed(func(network, addr string) {
		if network == tptypes.STCPR && addr != "" {
			data := appevent.TCPCloseData{RemoteNet: network, RemoteAddr: addr}
			event := appevent.NewEvent(appevent.TCPClose, data)
			if err := v.ebc.Broadcast(context.Background(), event); err != nil {
				v.log.WithError(err).Errorln("Failed to broadcast TCPClose event")
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		defer wg.Done()
		tpM.Serve(ctx)
	}()

	v.pushCloseStack("transport.manager", func() error {
		cancel()
		err := tpM.Close()
		wg.Wait()
		return err
	})

	v.initLock.Lock()
	v.tpM = tpM
	v.initLock.Unlock()
	return nil
}

func initTransportSetup(ctx context.Context, v *Visor, log *logging.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	ts, err := ts.NewTransportListener(ctx, v.conf, v.net.Dmsg(), v.tpM, v.MasterLogger())
	if err != nil {
		cancel()
		return err
	}
	go ts.Serve(ctx)
	v.pushCloseStack("transport_setup.rpc", func() error {
		cancel()
		return nil
	})
	return nil
}

func initRouter(ctx context.Context, v *Visor, log *logging.Logger) error {
	conf := v.conf.Routing
	rfClient := rfclient.NewHTTP(conf.RouteFinder, time.Duration(conf.RouteFinderTimeout))

	rConf := router.Config{
		Logger:           v.MasterLogger().PackageLogger("router"),
		PubKey:           v.conf.PK,
		SecKey:           v.conf.SK,
		TransportManager: v.tpM,
		RouteFinder:      rfClient,
		RouteGroupDialer: setupclient.NewSetupNodeDialer(),
		SetupNodes:       conf.SetupNodes,
		RulesGCInterval:  0, // TODO
		MinHops:          v.conf.Routing.MinHops,
	}

	r, err := router.New(v.net, &rConf)
	if err != nil {
		err := fmt.Errorf("failed to create router: %w", err)
		return err
	}

	// todo: this piece is somewhat ugly and inherited from the times when init was
	// calling init functions sequentially
	// It is probably a hack to run init
	// "somewhat concurrently", where the heaviest init functions will be partially concurrent

	// to avoid this we can:
	// either introduce some kind of "task" functionality that abstracts out
	// something that has to be run concurrent to the init, and check on their status
	// stop in close functions, etc

	// or, we can completely rely on the module system, and just wait for everything
	// in init functions, instead of spawning more goroutines.
	// but, even though modules themselves are concurrent this can introduce some
	// performance penalties, because dependencies will be waiting on complete init

	// leaving as it is until future requirements about init and modules are known

	serveCtx, cancel := context.WithCancel(context.Background())
	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		defer wg.Done()
		runtimeErrors := getErrors(ctx)
		if err := r.Serve(serveCtx); err != nil {
			runtimeErrors <- fmt.Errorf("serve router stopped: %w", err)
		}
	}()

	v.pushCloseStack("router.serve", func() error {
		cancel()
		err := r.Close()
		wg.Wait()
		return err
	})

	v.initLock.Lock()
	v.rfClient = rfClient
	v.router = r
	v.initLock.Unlock()

	return nil
}

func initLauncher(ctx context.Context, v *Visor, log *logging.Logger) error {
	conf := v.conf.Launcher

	// Prepare proc manager.
	procM, err := appserver.NewProcManager(v.MasterLogger(), &v.serviceDisc, v.ebc, conf.ServerAddr)
	if err != nil {
		err := fmt.Errorf("failed to start proc_manager: %w", err)
		return err
	}

	v.pushCloseStack("launcher.proc_manager", procM.Close)

	// Prepare launcher.
	launchConf := launcher.Config{
		VisorPK:    v.conf.PK,
		Apps:       conf.Apps,
		ServerAddr: conf.ServerAddr,
		BinPath:    conf.BinPath,
		LocalPath:  v.conf.LocalPath,
	}

	launchLog := v.MasterLogger().PackageLogger("launcher")

	launch, err := launcher.NewLauncher(launchLog, launchConf, v.net.Dmsg(), v.router, procM)
	if err != nil {
		err := fmt.Errorf("failed to start launcher: %w", err)
		return err
	}

	err = launch.AutoStart(launcher.EnvMap{
		skyenv.VPNClientName: vpnEnvMaker(v.conf, v.net, v.tpM.STCPRRemoteAddrs()),
		skyenv.VPNServerName: vpnEnvMaker(v.conf, v.net, nil),
	})

	if err != nil {
		err := fmt.Errorf("failed to autostart apps: %w", err)
		return err
	}

	v.initLock.Lock()
	v.procM = procM
	v.appL = launch
	v.initLock.Unlock()

	return nil
}

// Make an env maker function for vpn application
func vpnEnvMaker(conf *visorconfig.V1, n *snet.Network, tpRemoteAddrs []string) launcher.EnvMaker {
	return launcher.EnvMaker(func() ([]string, error) {
		var envCfg vpn.DirectRoutesEnvConfig

		if conf.Dmsg != nil {
			envCfg.DmsgDiscovery = conf.Dmsg.Discovery

			r := dmsgnetutil.NewRetrier(logrus.New(), 1*time.Second, 10*time.Second, 0, 1)
			err := r.Do(context.Background(), func() error {
				for _, ses := range n.Dmsg().AllSessions() {
					envCfg.DmsgServers = append(envCfg.DmsgServers, ses.RemoteTCPAddr().String())
				}

				if len(envCfg.DmsgServers) == 0 {
					return errors.New("no dmsg servers found")
				}

				return nil
			})

			if err != nil {
				return nil, fmt.Errorf("error getting Dmsg servers: %w", err)
			}
		}

		if conf.Transport != nil {
			envCfg.TPDiscovery = conf.Transport.Discovery
			envCfg.AddressResolver = conf.Transport.AddressResolver
		}

		if conf.Routing != nil {
			envCfg.RF = conf.Routing.RouteFinder
		}

		if conf.UptimeTracker != nil {
			envCfg.UptimeTracker = conf.UptimeTracker.Addr
		}

		if conf.STCP != nil && len(conf.STCP.PKTable) != 0 {
			envCfg.STCPTable = conf.STCP.PKTable
		}

		envCfg.TPRemoteIPs = tpRemoteAddrs

		envMap := vpn.AppEnvArgs(envCfg)

		envs := make([]string, 0, len(envMap))
		for k, v := range envMap {
			envs = append(envs, fmt.Sprintf("%s=%s", k, v))
		}

		return envs, nil
	})
}

func initCLI(ctx context.Context, v *Visor, log *logging.Logger) error {
	if v.conf.CLIAddr == "" {
		v.log.Info("'cli_addr' is not configured, skipping.")
		return nil
	}

	cliL, err := net.Listen("tcp", v.conf.CLIAddr)
	if err != nil {
		return err
	}

	v.pushCloseStack("cli.listener", cliL.Close)

	rpcS, err := newRPCServer(v, "CLI")
	if err != nil {
		err := fmt.Errorf("failed to start rpc server for cli: %w", err)
		return err
	}
	go rpcS.Accept(cliL) // We do not use sync.WaitGroup here as it will never return anyway.

	return nil
}

func initHypervisors(ctx context.Context, v *Visor, log *logging.Logger) error {
	hvErrs := make(map[cipher.PubKey]chan error, len(v.conf.Hypervisors))
	for _, hv := range v.conf.Hypervisors {
		hvErrs[hv] = make(chan error, 1)
	}

	for hvPK, hvErrs := range hvErrs {
		log := v.MasterLogger().PackageLogger("hypervisor_client").WithField("hypervisor_pk", hvPK)

		addr := dmsg.Addr{PK: hvPK, Port: skyenv.DmsgHypervisorPort}
		rpcS, err := newRPCServer(v, addr.PK.String()[:shortHashLen])
		if err != nil {
			err := fmt.Errorf("failed to start RPC server for hypervisor %s: %w", hvPK, err)
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		wg := new(sync.WaitGroup)
		wg.Add(1)

		go func(hvErrs chan error) {
			defer wg.Done()
			ServeRPCClient(ctx, log, v.net, rpcS, addr, hvErrs)
		}(hvErrs)

		v.pushCloseStack("hypervisor."+hvPK.String()[:shortHashLen], func() error {
			cancel()
			wg.Wait()
			return nil
		})
	}

	return nil
}

func initUptimeTracker(ctx context.Context, v *Visor, log *logging.Logger) error {
	const tickDuration = 1 * time.Minute

	conf := v.conf.UptimeTracker

	if conf == nil {
		v.log.Info("'uptime_tracker' is not configured, skipping.")
		return nil
	}

	ut, err := utclient.NewHTTP(conf.Addr, v.conf.PK, v.conf.SK)
	if err != nil {
		// TODO(evanlinjin): We should design utclient to retry automatically instead of returning error.
		v.log.WithError(err).Warn("Failed to connect to uptime tracker.")
		return nil
	}

	ticker := time.NewTicker(tickDuration)

	go func() {
		for range ticker.C {
			ctx := context.Background()
			if err := ut.UpdateVisorUptime(ctx); err != nil {
				log.WithError(err).Warn("Failed to update visor uptime.")
			}
		}
	}()

	v.pushCloseStack("uptime_tracker", func() error {
		ticker.Stop()
		return nil
	})

	v.initLock.Lock()
	v.uptimeTracker = ut
	v.initLock.Unlock()

	return nil
}

// this service is not considered critical and always returns true
// advertise this visor as public in service discovery
func initPublicVisor(_ context.Context, v *Visor, log *logging.Logger) error {
	if !v.conf.IsPublic {
		return nil
	}

	// retrieve interface IPs and check if at least one is public
	defaultIPs, err := netutil.DefaultNetworkInterfaceIPs()
	if err != nil {
		return nil
	}
	var found bool
	for _, IP := range defaultIPs {
		if netutil.IsPublicIP(IP) {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	// todo: consider moving this to transport into some helper function
	stcpr, ok := v.net.STcpr()
	if !ok {
		return nil
	}
	la, err := stcpr.LocalAddr()
	if err != nil {
		log.WithError(err).Errorln("Failed to get STCPR local addr")
		return nil
	}
	_, portStr, err := net.SplitHostPort(la.String())
	if err != nil {
		log.WithError(err).Errorf("Failed to extract port from addr %v", la.String())
		return nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.WithError(err).Errorf("Failed to convert port to int")
		return nil
	}

	visorUpdater := v.serviceDisc.VisorUpdater(uint16(port))
	visorUpdater.Start()

	v.log.Infof("Sent request to register visor as public")
	v.pushCloseStack("visor updater", func() error {
		visorUpdater.Stop()
		return nil
	})
	return nil
}

func initPublicVisors(ctx context.Context, v *Visor, log *logging.Logger) error {
	if !v.conf.Transport.AutoconnectPublic {
		return nil
	}
	proxyDisc := v.conf.Launcher.Discovery.ServiceDisc
	if proxyDisc == "" {
		proxyDisc = skyenv.DefaultServiceDiscAddr
	}

	// todo: refactor appdisc: split connecting to services in appdisc and
	// advertising oneself as a service. Currently, config is tailored to
	// advertising oneself and requires things like port that are not used
	// in connecting to services
	conf := servicedisc.Config{
		Type:     servicedisc.ServiceTypeVisor,
		PK:       v.conf.PK,
		SK:       v.conf.SK,
		Port:     uint16(0),
		DiscAddr: proxyDisc,
	}
	connector := servicedisc.MakeConnector(conf, 5, v.tpM, log)
	go connector.Run(ctx) //nolint:errcheck

	return nil
}

func initHypervisor(_ context.Context, v *Visor, log *logging.Logger) error {
	v.log.Infof("Initializing hypervisor")

	ctx, cancel := context.WithCancel(context.Background())

	conf := *v.conf.Hypervisor
	conf.PK = v.conf.PK
	conf.SK = v.conf.SK
	conf.DmsgDiscovery = v.conf.Dmsg.Discovery

	// Prepare hypervisor.
	hv, err := New(conf, v, v.net.Dmsg())
	if err != nil {
		v.log.Fatalln("Failed to start hypervisor:", err)
	}

	hv.serveDmsg(ctx, v.log)

	// Serve HTTP(s).
	v.log.WithField("addr", conf.HTTPAddr).
		WithField("tls", conf.EnableTLS).
		Info("Serving hypervisor...")

	go func() {
		if handler := hv.HTTPHandler(); conf.EnableTLS {
			err = http.ListenAndServeTLS(conf.HTTPAddr, conf.TLSCertFile, conf.TLSKeyFile, handler)
		} else {
			err = http.ListenAndServe(conf.HTTPAddr, handler)
		}

		if err != nil {
			v.log.WithError(err).Fatal("Hypervisor exited with error.")
		}

		cancel()
	}()

	v.log.Infof("Hypervisor initialized")

	return nil
}

func connectToTpDisc(v *Visor) (transport.DiscoveryClient, error) {
	const (
		initBO = 1 * time.Second
		maxBO  = 10 * time.Second
		// trying till success
		tries  = 0
		factor = 1
	)

	conf := v.conf.Transport

	log := v.MasterLogger().PackageLogger("tp_disc_retrier")
	tpdCRetrier := dmsgnetutil.NewRetrier(log,
		initBO, maxBO, tries, factor)

	var tpdC transport.DiscoveryClient
	retryFunc := func() error {
		var err error
		tpdC, err = tpdclient.NewHTTP(conf.Discovery, v.conf.PK, v.conf.SK)
		if err != nil {
			log.WithError(err).Error("Failed to connect to transport discovery, retrying...")
			return err
		}

		return nil
	}

	if err := tpdCRetrier.Do(context.Background(), retryFunc); err != nil {
		return nil, err
	}

	return tpdC, nil
}

// ErrNoVisorInCtx is returned when visor is not set in module initialization context
var ErrNoVisorInCtx = errors.New("visor not set in module initialization context")

// ErrNoErrorsCtx is returned when errors channel is not set in module initialization context
var ErrNoErrorsCtx = errors.New("errors not set in module initialization context")

// withInitCtx wraps init function and returns a hook that can be used in
// the module system
// Passed context should have visor value under visorKey key, this visor will be used
// in the passed function
// Passed context should have errors channel for module runtime errors. It can be accessed
// through a function call
func withInitCtx(f initFn) vinit.Hook {
	return func(ctx context.Context, log *logging.Logger) error {
		val := ctx.Value(visorKey)
		v, ok := val.(*Visor)
		if !ok && v == nil {
			return ErrNoVisorInCtx
		}
		val = ctx.Value(runtimeErrsKey)
		errs, ok := val.(chan error)
		if !ok && errs == nil {
			return ErrNoErrorsCtx
		}
		return f(ctx, v, log)
	}
}

func getErrors(ctx context.Context) chan error {
	val := ctx.Value(runtimeErrsKey)
	errs, ok := val.(chan error)
	if !ok && errs == nil {
		// ok to panic because with check for this value in withInitCtx
		// probably will never be reached, but better than generic NPE just in case
		panic("runtime errors channel is not set in context")
	}
	return errs
}
