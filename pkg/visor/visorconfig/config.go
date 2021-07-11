package visorconfig

import (
	"runtime"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/pkg/app/launcher"
	"github.com/skycoin/skywire/pkg/dmsgc"
	"github.com/skycoin/skywire/pkg/restart"
	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/skyenv"
	"github.com/skycoin/skywire/pkg/transport/network"
	"github.com/skycoin/skywire/pkg/visor/hypervisorconfig"
)

var (
	appDefaultConfigs = map[string]launcher.AppConfig{
		skyenv.SkychatName: {
			Name:      skyenv.SkychatName,
			AutoStart: true,
			Port:      routing.Port(skyenv.SkychatPort),
			Args:      []string{"-addr", skyenv.SkychatAddr},
		},
		skyenv.SkysocksName: {
			Name:      skyenv.SkysocksName,
			AutoStart: true,
			Port:      routing.Port(skyenv.SkysocksPort),
		},
		skyenv.SkysocksClientName: {
			Name:      skyenv.SkysocksClientName,
			AutoStart: false,
			Port:      routing.Port(skyenv.SkysocksClientPort),
		},
		skyenv.VPNServerName: {
			Name:      skyenv.VPNServerName,
			AutoStart: false,
			Port:      routing.Port(skyenv.VPNServerPort),
		},
		skyenv.VPNClientName: {
			Name:      skyenv.VPNClientName,
			AutoStart: false,
			Port:      routing.Port(skyenv.VPNClientPort),
		},
	}
)

// MakeBaseConfig returns a visor config with 'enforced' fields only.
// This is used as default values if no config is given, or for missing *required* fields.
// This function always returns the latest config version.
func MakeBaseConfig(common *Common) *V1 {
	conf := new(V1)
	conf.Common = common
	conf.Dmsg = &dmsgc.DmsgConfig{
		Discovery:     skyenv.DefaultDmsgDiscAddr,
		SessionsCount: 1,
	}
	conf.Transport = &V1Transport{
		Discovery:       skyenv.DefaultTpDiscAddr,
		AddressResolver: skyenv.DefaultAddressResolverAddr,
	}
	conf.Routing = &V1Routing{
		SetupNodes:         []cipher.PubKey{skyenv.MustPK(skyenv.DefaultSetupPK)},
		RouteFinder:        skyenv.DefaultRouteFinderAddr,
		RouteFinderTimeout: DefaultTimeout,
	}
	conf.Launcher = &V1Launcher{
		Discovery: &V1AppDisc{
			ServiceDisc: skyenv.DefaultServiceDiscAddr,
		},
		Apps:       nil,
		ServerAddr: skyenv.DefaultAppSrvAddr,
		BinPath:    skyenv.DefaultAppBinPath,
	}
	conf.UptimeTracker = &V1UptimeTracker{
		Addr: skyenv.DefaultUptimeTrackerAddr,
	}
	conf.CLIAddr = skyenv.DefaultRPCAddr
	conf.LogLevel = skyenv.DefaultLogLevel
	conf.LocalPath = skyenv.DefaultLocalPath
	conf.ShutdownTimeout = DefaultTimeout
	conf.RestartCheckDelay = Duration(restart.DefaultCheckDelay)
	return conf
}

// MakeDefaultConfig returns the default visor config from a given secret key (if specified).
// The config's 'sk' field will be nil if not specified.
// Generated config will be saved to 'confPath'.
// This function always returns the latest config version.
func MakeDefaultConfig(log *logging.MasterLogger, confPath string, sk *cipher.SecKey, hypervisor bool,
	genAppConfig map[string]bool) (*V1, error) {
	cc, err := NewCommon(log, confPath, V1Name, sk)
	if err != nil {
		return nil, err
	}
	return defaultConfigFromCommon(cc, hypervisor, genAppConfig)
}

func defaultConfigFromCommon(cc *Common, hypervisor bool, genAppConfig map[string]bool) (*V1, error) {
	// Enforce version and keys in 'cc'.
	cc.Version = V1Name
	if err := cc.ensureKeys(); err != nil {
		return nil, err
	}

	// Actual config generation.
	conf := MakeBaseConfig(cc)

	conf.Dmsgpty = &V1Dmsgpty{
		Port:    skyenv.DmsgPtyPort,
		CLINet:  skyenv.DefaultDmsgPtyCLINet,
		CLIAddr: skyenv.DefaultDmsgPtyCLIAddr,
	}

	conf.STCP = &network.STCPConfig{
		LocalAddr: skyenv.DefaultSTCPAddr,
		PKTable:   nil,
	}

	conf.UptimeTracker = &V1UptimeTracker{
		Addr: skyenv.DefaultUptimeTrackerAddr,
	}

	conf.Launcher.Discovery = &V1AppDisc{
		UpdateInterval: Duration(skyenv.AppDiscUpdateInterval),
		ServiceDisc:    skyenv.DefaultServiceDiscAddr,
	}

	for appName, gen := range genAppConfig {
		if gen {
			if appConf, knownApp := appDefaultConfigs[appName]; knownApp {
				conf.Launcher.Apps = append(conf.Launcher.Apps, appConf)
			}
		}
	}

	conf.Hypervisors = make([]cipher.PubKey, 0)

	if hypervisor {
		config := hypervisorconfig.GenerateWorkDirConfig(false)
		conf.Hypervisor = &config
	}

	return conf, nil
}

// MakeTestConfig acts like MakeDefaultConfig, however, test deployment service addresses are used instead.
func MakeTestConfig(log *logging.MasterLogger, confPath string, sk *cipher.SecKey, hypervisor bool,
	genAppConfig map[string]bool) (*V1, error) {
	conf, err := MakeDefaultConfig(log, confPath, sk, hypervisor, genAppConfig)
	if err != nil {
		return nil, err
	}
	SetDefaultTestingValues(conf)
	if conf.Hypervisor != nil {
		conf.Hypervisor.DmsgDiscovery = conf.Transport.Discovery
	}

	return conf, nil
}

// MakePackageConfig acts like MakeDefaultConfig but use package config defaults
func MakePackageConfig(log *logging.MasterLogger, confPath string, sk *cipher.SecKey, hypervisor bool,
	genAppConfig map[string]bool) (*V1, error) {
	conf, err := MakeDefaultConfig(log, confPath, sk, hypervisor, genAppConfig)
	if err != nil {
		return nil, err
	}

	conf.Dmsgpty = &V1Dmsgpty{
		Port:    skyenv.DmsgPtyPort,
		CLINet:  skyenv.DefaultDmsgPtyCLINet,
		CLIAddr: skyenv.DefaultDmsgPtyCLIAddr,
	}
	conf.LocalPath = skyenv.PackageAppLocalPath()
	conf.Launcher.BinPath = skyenv.PackageAppBinPath()

	if conf.Hypervisor != nil {
		conf.Hypervisor.EnableAuth = skyenv.DefaultEnableAuth
		conf.Hypervisor.EnableTLS = skyenv.PackageEnableTLS
		if runtime.GOOS == "darwin" {
			// disable TLS by default for OSX
			conf.Hypervisor.EnableTLS = false
		}
		conf.Hypervisor.TLSKeyFile = skyenv.PackageTLSKey()
		conf.Hypervisor.TLSCertFile = skyenv.PackageTLSCert()
	}
	return conf, nil
}

// SetDefaultTestingValues mutates configuration to use testing values
func SetDefaultTestingValues(conf *V1) {
	conf.Dmsg.Discovery = skyenv.TestDmsgDiscAddr
	conf.Transport.Discovery = skyenv.TestTpDiscAddr
	conf.Transport.AddressResolver = skyenv.TestAddressResolverAddr
	conf.Routing.RouteFinder = skyenv.TestRouteFinderAddr
	conf.Routing.SetupNodes = []cipher.PubKey{skyenv.MustPK(skyenv.TestSetupPK)}
	conf.UptimeTracker.Addr = skyenv.TestUptimeTrackerAddr
	conf.Launcher.Discovery.ServiceDisc = skyenv.TestServiceDiscAddr
}

// SetDefaultProductionValues mutates configuration to use production values
func SetDefaultProductionValues(conf *V1) {
	conf.Dmsg.Discovery = skyenv.DefaultDmsgDiscAddr
	conf.Transport.Discovery = skyenv.DefaultTpDiscAddr
	conf.Transport.AddressResolver = skyenv.DefaultAddressResolverAddr
	conf.Routing.RouteFinder = skyenv.DefaultRouteFinderAddr
	conf.Routing.SetupNodes = []cipher.PubKey{skyenv.MustPK(skyenv.DefaultSetupPK)}
	conf.UptimeTracker = &V1UptimeTracker{
		Addr: skyenv.DefaultUptimeTrackerAddr,
	}
	conf.Launcher.Discovery = &V1AppDisc{
		UpdateInterval: Duration(skyenv.AppDiscUpdateInterval),
		ServiceDisc:    skyenv.DefaultServiceDiscAddr,
	}
}
