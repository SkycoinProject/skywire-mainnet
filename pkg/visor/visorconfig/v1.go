package visorconfig

import (
	"fmt"
	"strings"
	"sync"

	"github.com/skycoin/dmsg/cipher"

	"github.com/skycoin/skywire/pkg/app/launcher"
	"github.com/skycoin/skywire/pkg/dmsgc"
	"github.com/skycoin/skywire/pkg/transport/network"
	"github.com/skycoin/skywire/pkg/visor/hypervisorconfig"
)

//go:generate readmegen -n V1 -o ./README.md ./v1.go

// V100Name is the semantic version string for v1.0.0.
const V100Name = "v1.0.0"

// V101Name is the semantic version string for v1.0.1.
const V101Name = "v1.0.1"

// V110Name is the semantic version string for v1.1.0.
// Added MinHops field to V1Routing section of config
// Removed public_trusted_visor field from root section
// Removed trusted_visors field from transport section
// Added is_public field to root section
// Added public_autoconnect field to transport section
// Added transport_setup_nodes field to transport section
// Removed authorization_file field from dmsgpty section
// Default urls are changed to newer shortned ones
// Added stun_servers field to the config
const V110Name = "v1.1.0"

// V1Name is the semantic version string for the most recent version of V1.
const V1Name = V110Name

// V1 is visor config
type V1 struct {
	*Common
	mu sync.RWMutex

	Dmsg          *dmsgc.DmsgConfig   `json:"dmsg"`
	Dmsgpty       *V1Dmsgpty          `json:"dmsgpty,omitempty"`
	STCP          *network.STCPConfig `json:"stcp,omitempty"`
	Transport     *V1Transport        `json:"transport"`
	Routing       *V1Routing          `json:"routing"`
	UptimeTracker *V1UptimeTracker    `json:"uptime_tracker,omitempty"`
	Launcher      *V1Launcher         `json:"launcher"`

	Hypervisors []cipher.PubKey `json:"hypervisors"`
	CLIAddr     string          `json:"cli_addr"`

	LogLevel          string   `json:"log_level"`
	LocalPath         string   `json:"local_path"`
	StunServers       []string `json:"stun_servers"`
	ShutdownTimeout   Duration `json:"shutdown_timeout,omitempty"`    // time value, examples: 10s, 1m, etc
	RestartCheckDelay Duration `json:"restart_check_delay,omitempty"` // time value, examples: 10s, 1m, etc
	IsPublic          bool     `json:"is_public"`

	Hypervisor *hypervisorconfig.Config `json:"hypervisor,omitempty"`
}

// V1Dmsgpty configures the dmsgpty-host.
type V1Dmsgpty struct {
	Port    uint16 `json:"port"`
	CLINet  string `json:"cli_network"`
	CLIAddr string `json:"cli_address"`
}

// V1Transport defines a transport config.
type V1Transport struct {
	Discovery         string          `json:"discovery"`
	AddressResolver   string          `json:"address_resolver"`
	AutoconnectPublic bool            `json:"public_autoconnect"`
	TransportSetup    []cipher.PubKey `json:"transport_setup_nodes"`
}

// V1LogStore configures a LogStore.
type V1LogStore struct {
	// Type defines the log store type. Valid values: file, memory.
	Type     string `json:"type"`
	Location string `json:"location"`
}

// V1Routing configures routing.
type V1Routing struct {
	SetupNodes         []cipher.PubKey `json:"setup_nodes,omitempty"`
	RouteFinder        string          `json:"route_finder"`
	RouteFinderTimeout Duration        `json:"route_finder_timeout,omitempty"`
	MinHops            uint16          `json:"min_hops"`
}

// V1UptimeTracker configures uptime tracker.
type V1UptimeTracker struct {
	Addr string `json:"addr"`
}

// V1AppDisc configures Skywire App Discovery Clients.
type V1AppDisc struct {
	UpdateInterval Duration `json:"update_interval,omitempty"`
	ServiceDisc    string   `json:"proxy_discovery_addr"` // TODO: change JSON name
}

// V1Launcher configures the app launcher.
type V1Launcher struct {
	Discovery  *V1AppDisc           `json:"discovery"`
	Apps       []launcher.AppConfig `json:"apps"`
	ServerAddr string               `json:"server_addr"`
	BinPath    string               `json:"bin_path"`
}

// Flush flushes the config to file (if specified).
func (v1 *V1) Flush() error {
	v1.mu.Lock()
	defer v1.mu.Unlock()

	return v1.Common.flush(v1)
}

// UpdateAppAutostart modifies a single app's autostart value within the config and also the given launcher.
// The updated config gets flushed to file if there are any changes.
func (v1 *V1) UpdateAppAutostart(launch *launcher.Launcher, appName string, autoStart bool) error {
	v1.mu.Lock()
	defer v1.mu.Unlock()

	conf := v1.Launcher

	changed := false
	for i := range conf.Apps {
		if conf.Apps[i].Name == appName {
			conf.Apps[i].AutoStart = autoStart
			changed = true
			break
		}
	}

	if !changed {
		return nil
	}

	launch.ResetConfig(launcher.Config{
		VisorPK:    v1.PK,
		Apps:       conf.Apps,
		ServerAddr: conf.ServerAddr,
	})
	return v1.flush(v1)
}

// UpdateAppArg updates the cli flag of the specified app config and also within the launcher.
// The updated config gets flushed to file if there are any changes.
func (v1 *V1) UpdateAppArg(launch *launcher.Launcher, appName, argName string, value interface{}) error {
	v1.mu.Lock()
	defer v1.mu.Unlock()

	conf := v1.Launcher

	var configChanged bool
	switch val := value.(type) {
	case string:
		configChanged = updateStringArg(conf, appName, argName, val)
	case bool:
		configChanged = updateBoolArg(conf, appName, argName, val)
	default:
		return fmt.Errorf("invalid arg type %T", value)
	}

	if !configChanged {
		return nil
	}

	launch.ResetConfig(launcher.Config{
		VisorPK:    v1.PK,
		Apps:       conf.Apps,
		ServerAddr: conf.ServerAddr,
	})

	return v1.flush(v1)
}

// UpdateMinHops updates min_hops config
func (v1 *V1) UpdateMinHops(hops uint16) error {
	v1.mu.Lock()
	v1.Routing.MinHops = hops
	v1.mu.Unlock()

	return v1.flush(v1)
}

// updateStringArg updates the cli non-boolean flag of the specified app config and also within the launcher.
// It removes argName from app args if value is an empty string.
// The updated config gets flushed to file if there are any changes.
func updateStringArg(conf *V1Launcher, appName, argName, value string) bool {
	configChanged := false

	for i := range conf.Apps {
		if conf.Apps[i].Name != appName {
			continue
		}

		configChanged = true

		argChanged := false
		l := len(conf.Apps[i].Args)
		for j := 0; j < l; j++ {
			equalArgName := conf.Apps[i].Args[j] == argName && j+1 < len(conf.Apps[i].Args)
			if !equalArgName {
				continue
			}

			if value == "" {
				conf.Apps[i].Args = append(conf.Apps[i].Args[:j], conf.Apps[i].Args[j+2:]...)
				j-- //nolint:ineffassign
			} else {
				conf.Apps[i].Args[j+1] = value
			}

			argChanged = true
			break
		}

		if !argChanged && value != "" {
			conf.Apps[i].Args = append(conf.Apps[i].Args, argName, value)
		}

		break
	}

	return configChanged
}

// updateBoolArg updates the cli boolean flag of the specified app config and also within the launcher.
// All flag names and values are formatted as "-name=value" to allow arbitrary values with respect to different
// possible default values.
// The updated config gets flushed to file if there are any changes.
func updateBoolArg(conf *V1Launcher, appName, argName string, value bool) bool {
	const argFmt = "%s=%v"

	configChanged := false

	for i := range conf.Apps {
		if conf.Apps[i].Name != appName {
			continue
		}

		// we format it to have a single dash, just to unify representation
		fmtedArgName := argName
		if argName[1] == '-' {
			fmtedArgName = fmtedArgName[1:]
		}

		arg := fmt.Sprintf(argFmt, fmtedArgName, value)

		configChanged = true

		argChanged := false
		for j := 0; j < len(conf.Apps[i].Args); j++ {
			// there shouldn't be such values if config is modified automatically,
			// but might happen if done manually, so we avoid further panic with this check
			if len(conf.Apps[i].Args[j]) < 2 {
				continue
			}

			equalArgName := conf.Apps[i].Args[j][1] != '-' && strings.HasPrefix(conf.Apps[i].Args[j], fmtedArgName)
			if conf.Apps[i].Args[j][1] == '-' {
				equalArgName = strings.HasPrefix(conf.Apps[i].Args[j], "-"+fmtedArgName)
			}

			if !equalArgName {
				continue
			}

			// check next value. currently we store value along with the flag name in a single string,
			// but there're may be some broken configs because of the previous functionality, so we
			// make our best effort to fix this on the go
			if (j + 1) < len(conf.Apps[i].Args) {
				// bool value shouldn't be present there, so we remove it, if it is
				if conf.Apps[i].Args[j+1] == "true" || conf.Apps[i].Args[j+1] == "false" {
					if (j + 2) < len(conf.Apps[i].Args) {
						conf.Apps[i].Args = append(conf.Apps[i].Args[:j+1], conf.Apps[i].Args[j+2:]...)
					} else {
						conf.Apps[i].Args = conf.Apps[i].Args[:j+1]
					}
				}
			}

			conf.Apps[i].Args[j] = arg
			argChanged = true

			break
		}

		if !argChanged {
			conf.Apps[i].Args = append(conf.Apps[i].Args, arg)
		}

		break
	}

	return configChanged
}
