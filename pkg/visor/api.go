package visor

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ccding/go-stun/stun"
	"github.com/google/uuid"
	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cipher"

	"github.com/skycoin/skywire/pkg/app/appserver"
	"github.com/skycoin/skywire/pkg/app/launcher"
	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/skyenv"
	"github.com/skycoin/skywire/pkg/transport"
	"github.com/skycoin/skywire/pkg/transport/network"
	"github.com/skycoin/skywire/pkg/util/netutil"
	"github.com/skycoin/skywire/pkg/util/updater"
	"github.com/skycoin/skywire/pkg/visor/dmsgtracker"
)

// API represents visor API.
type API interface {
	Overview() (*Overview, error)
	Summary() (*Summary, error)

	Health() (*HealthInfo, error)
	Uptime() (float64, error)

	Apps() ([]*launcher.AppState, error)
	StartApp(appName string) error
	StopApp(appName string) error
	SetAppDetailedStatus(appName, state string) error
	RestartApp(appName string) error
	SetAutoStart(appName string, autostart bool) error
	SetAppPassword(appName, password string) error
	SetAppPK(appName string, pk cipher.PubKey) error
	SetAppSecure(appName string, isSecure bool) error
	SetAppKillswitch(appName string, killswitch bool) error
	LogsSince(timestamp time.Time, appName string) ([]string, error)
	GetAppStats(appName string) (appserver.AppStats, error)
	GetAppConnectionsSummary(appName string) ([]appserver.ConnectionSummary, error)

	TransportTypes() ([]string, error)
	Transports(types []string, pks []cipher.PubKey, logs bool) ([]*TransportSummary, error)
	Transport(tid uuid.UUID) (*TransportSummary, error)
	AddTransport(remote cipher.PubKey, tpType string, public bool, timeout time.Duration) (*TransportSummary, error)
	RemoveTransport(tid uuid.UUID) error

	DiscoverTransportsByPK(pk cipher.PubKey) ([]*transport.EntryWithStatus, error)
	DiscoverTransportByID(id uuid.UUID) (*transport.EntryWithStatus, error)

	RoutingRules() ([]routing.Rule, error)
	RoutingRule(key routing.RouteID) (routing.Rule, error)
	SaveRoutingRule(rule routing.Rule) error
	RemoveRoutingRule(key routing.RouteID) error

	RouteGroups() ([]RouteGroupInfo, error)

	Restart() error
	Exec(command string) ([]byte, error)
	Update(config updater.UpdateConfig) (bool, error)
	UpdateWithStatus(config updater.UpdateConfig) <-chan StatusMessage
	UpdateAvailable(channel updater.Channel) (*updater.Version, error)
	UpdateStatus() (string, error)
	RuntimeLogs() (string, error)

	SetMinHops(uint16) error
}

// HealthCheckable resource returns its health status as an integer
// that corresponds to HTTP status code returned from the resource
// 200 codes correspond to a healthy resource
type HealthCheckable interface {
	Health(ctx context.Context) (int, error)
}

// Overview provides a range of basic information about a Visor.
type Overview struct {
	PubKey          cipher.PubKey        `json:"local_pk"`
	BuildInfo       *buildinfo.Info      `json:"build_info"`
	AppProtoVersion string               `json:"app_protocol_version"`
	Apps            []*launcher.AppState `json:"apps"`
	Transports      []*TransportSummary  `json:"transports"`
	RoutesCount     int                  `json:"routes_count"`
	LocalIP         string               `json:"local_ip"`
	PublicIP        string               `json:"public_ip"`
	IsSymmetricNAT  bool                 `json:"is_symmetic_nat"`
}

// Overview implements API.
func (v *Visor) Overview() (*Overview, error) {
	var tSummaries []*TransportSummary
	var publicIP string
	var isSymmetricNAT bool
	if v == nil {
		panic("v is nil")
	}
	if v.tpM == nil {
		panic("tpM is nil")
	}
	v.tpM.WalkTransports(func(tp *transport.ManagedTransport) bool {
		tSummaries = append(tSummaries,
			newTransportSummary(v.tpM, tp, true, v.router.SetupIsTrusted(tp.Remote())))
		return true
	})
	if v.stunClient != nil {
		switch v.stunClient.NATType {
		case stun.NATNone, stun.NATFull, stun.NATRestricted, stun.NATPortRestricted:
			publicIP = v.stunClient.PublicIP.IP()
			isSymmetricNAT = true
		case stun.NATSymmetric, stun.NATSymmetricUDPFirewall:
			isSymmetricNAT = false
		}
	}

	overview := &Overview{
		PubKey:          v.conf.PK,
		BuildInfo:       buildinfo.Get(),
		AppProtoVersion: supportedProtocolVersion,
		Apps:            v.appL.AppStates(),
		Transports:      tSummaries,
		RoutesCount:     v.router.RoutesCount(),
		PublicIP:        publicIP,
		IsSymmetricNAT:  isSymmetricNAT,
	}

	localIPs, err := netutil.DefaultNetworkInterfaceIPs()
	if err != nil {
		return nil, err
	}

	if len(localIPs) > 0 {
		// should be okay to have the first one, in the case of
		// active network interface, there's usually just a single IP
		overview.LocalIP = localIPs[0].String()
	}

	return overview, nil
}

// Summary provides detailed info including overview and health of the visor.
type Summary struct {
	Overview     *Overview                      `json:"overview"`
	Health       *HealthInfo                    `json:"health"`
	Uptime       float64                        `json:"uptime"`
	Routes       []routingRuleResp              `json:"routes"`
	IsHypervisor bool                           `json:"is_hypervisor,omitempty"`
	DmsgStats    *dmsgtracker.DmsgClientSummary `json:"dmsg_stats"`
	Online       bool                           `json:"online"`
	MinHops      uint16                         `json:"min_hops"`
}

// Summary implements API.
func (v *Visor) Summary() (*Summary, error) {
	overview, err := v.Overview()
	if err != nil {
		return nil, fmt.Errorf("overview")
	}

	health, err := v.Health()
	if err != nil {
		return nil, fmt.Errorf("health")
	}

	uptime, err := v.Uptime()
	if err != nil {
		return nil, fmt.Errorf("uptime")
	}

	routes, err := v.RoutingRules()
	if err != nil {
		return nil, fmt.Errorf("routes")
	}

	extraRoutes := make([]routingRuleResp, 0, len(routes))
	for _, route := range routes {
		extraRoutes = append(extraRoutes, routingRuleResp{
			Key:     route.KeyRouteID(),
			Rule:    hex.EncodeToString(route),
			Summary: route.Summary(),
		})
	}

	summary := &Summary{
		Overview: overview,
		Health:   health,
		Uptime:   uptime,
		Routes:   extraRoutes,
		MinHops:  v.conf.Routing.MinHops,
	}

	return summary, nil
}

// collectHealthStats for given services and return health statuses
func (v *Visor) collectHealthStats(services map[string]HealthCheckable) map[string]int {
	type healthResponse struct {
		name   string
		status int
	}

	ctx, cancel := context.WithTimeout(context.Background(), InnerHealthTimeout)
	defer cancel()

	responses := make(chan healthResponse, len(services))

	var wg sync.WaitGroup
	wg.Add(len(services))
	for name, service := range services {
		go func(name string, service HealthCheckable) {
			defer wg.Done()

			if service == nil {
				responses <- healthResponse{name, http.StatusNotFound}
				return
			}

			status, err := service.Health(ctx)
			if err != nil {
				v.log.WithError(err).Warnf("Failed to check service health, service name: %s", name)
				status = http.StatusInternalServerError
			}

			responses <- healthResponse{name, status}
		}(name, service)
	}
	wg.Wait()

	close(responses)

	results := make(map[string]int)
	for response := range responses {
		results[response.name] = response.status
	}

	return results
}

// HealthInfo carries information about visor's external services health represented as http status codes
type HealthInfo struct {
	TransportDiscovery int `json:"transport_discovery"`
	RouteFinder        int `json:"route_finder"`
	SetupNode          int `json:"setup_node"`
	UptimeTracker      int `json:"uptime_tracker"`
	AddressResolver    int `json:"address_resolver"`
}

// Health implements API.
func (v *Visor) Health() (*HealthInfo, error) {
	services := map[string]HealthCheckable{
		"td": v.tpDiscClient(),
		"rf": v.routeFinderClient(),
		"ut": v.uptimeTrackerClient(),
		"ar": v.addressResolverClient(),
	}

	stats := v.collectHealthStats(services)
	healthInfo := &HealthInfo{
		TransportDiscovery: stats["td"],
		RouteFinder:        stats["rf"],
		UptimeTracker:      stats["ut"],
		AddressResolver:    stats["ar"],
	}
	// TODO(evanlinjin): This should actually poll the setup nodes services.
	if len(v.conf.Routing.SetupNodes) == 0 {
		healthInfo.SetupNode = http.StatusNotFound
	} else {
		healthInfo.SetupNode = http.StatusOK
	}

	return healthInfo, nil
}

// Uptime implements API.
func (v *Visor) Uptime() (float64, error) {
	return time.Since(v.startedAt).Seconds(), nil
}

// Apps implements API.
func (v *Visor) Apps() ([]*launcher.AppState, error) {
	return v.appL.AppStates(), nil
}

// StartApp implements API.
func (v *Visor) StartApp(appName string) error {
	var envs []string
	var err error
	if appName == skyenv.VPNClientName {
		// todo: can we use some kind of app start hook that will be used for both autostart
		// and start? Reason: this is also called in init for autostart
		maker := vpnEnvMaker(v.conf, v.dmsgC, v.tpM.STCPRRemoteAddrs())
		envs, err = maker()
		if err != nil {
			return err
		}
	}

	return v.appL.StartApp(appName, nil, envs)
}

// StopApp implements API.
func (v *Visor) StopApp(appName string) error {
	_, err := v.appL.StopApp(appName)
	return err
}

// SetAppDetailedStatus implements API.
func (v *Visor) SetAppDetailedStatus(appName, status string) error {
	proc, ok := v.procM.ProcByName(appName)
	if !ok {
		return ErrAppProcNotRunning
	}

	v.log.Infof("Setting app detailed status %v for app %v", status, appName)
	proc.SetDetailedStatus(status)

	return nil
}

// RestartApp implements API.
func (v *Visor) RestartApp(appName string) error {
	if _, ok := v.procM.ProcByName(appName); ok {
		v.log.Infof("Updated %v password, restarting it", appName)
		return v.appL.RestartApp(appName)
	}

	return nil
}

// SetAutoStart implements API.
func (v *Visor) SetAutoStart(appName string, autoStart bool) error {
	if _, ok := v.appL.AppState(appName); !ok {
		return ErrAppProcNotRunning
	}

	v.log.Infof("Saving auto start = %v for app %v to config", autoStart, appName)
	return v.conf.UpdateAppAutostart(v.appL, appName, autoStart)
}

// SetAppPassword implements API.
func (v *Visor) SetAppPassword(appName, password string) error {
	allowedToChangePassword := func(appName string) bool {
		allowedApps := map[string]struct{}{
			skyenv.SkysocksName:  {},
			skyenv.VPNClientName: {},
			skyenv.VPNServerName: {},
		}

		_, ok := allowedApps[appName]
		return ok
	}

	if !allowedToChangePassword(appName) {
		return fmt.Errorf("app %s is not allowed to change password", appName)
	}

	v.log.Infof("Changing %s password to %q", appName, password)

	const (
		passcodeArgName = "-passcode"
	)

	if err := v.conf.UpdateAppArg(v.appL, appName, passcodeArgName, password); err != nil {
		return err
	}

	v.log.Infof("Updated %v password", appName)

	return nil
}

// SetAppKillswitch implements API.
func (v *Visor) SetAppKillswitch(appName string, killswitch bool) error {
	if appName != skyenv.VPNClientName {
		return fmt.Errorf("app %s is not allowed to set killswitch", appName)
	}

	v.log.Infof("Setting %s killswitch to %q", appName, killswitch)

	const (
		killSwitchArg = "--killswitch"
	)

	if err := v.conf.UpdateAppArg(v.appL, appName, killSwitchArg, killswitch); err != nil {
		return err
	}

	v.log.Infof("Updated %v killswitch state", appName)

	return nil
}

// SetAppSecure implements API.
func (v *Visor) SetAppSecure(appName string, isSecure bool) error {
	if appName != skyenv.VPNServerName {
		return fmt.Errorf("app %s is not allowed to change 'secure' parameter", appName)
	}

	v.log.Infof("Setting %s secure to %q", appName, isSecure)

	const (
		secureArgName = "--secure"
	)

	if err := v.conf.UpdateAppArg(v.appL, appName, secureArgName, isSecure); err != nil {
		return err
	}

	v.log.Infof("Updated %v secure state", appName)

	return nil
}

// SetAppPK implements API.
func (v *Visor) SetAppPK(appName string, pk cipher.PubKey) error {
	allowedToChangePK := func(appName string) bool {
		allowedApps := map[string]struct{}{
			skyenv.SkysocksClientName: {},
			skyenv.VPNClientName:      {},
		}

		_, ok := allowedApps[appName]
		return ok
	}

	if !allowedToChangePK(appName) {
		return fmt.Errorf("app %s is not allowed to change PK", appName)
	}

	v.log.Infof("Changing %s PK to %q", appName, pk)

	const (
		pkArgName = "-srv"
	)

	if err := v.conf.UpdateAppArg(v.appL, appName, pkArgName, pk.String()); err != nil {
		return err
	}

	v.log.Infof("Updated %v PK", appName)

	return nil
}

// LogsSince implements API.
func (v *Visor) LogsSince(timestamp time.Time, appName string) ([]string, error) {
	proc, ok := v.procM.ProcByName(appName)
	if !ok {
		return nil, fmt.Errorf("proc of app name '%s' is not found", appName)
	}

	res, err := proc.Logs().LogsSince(timestamp)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// GetAppStats implements API.
func (v *Visor) GetAppStats(appName string) (appserver.AppStats, error) {
	stats, err := v.procM.Stats(appName)
	if err != nil {
		return appserver.AppStats{}, err
	}

	return stats, nil
}

// GetAppConnectionsSummary implements API.
func (v *Visor) GetAppConnectionsSummary(appName string) ([]appserver.ConnectionSummary, error) {
	cSummary, err := v.procM.ConnectionsSummary(appName)
	if err != nil {
		return nil, err
	}

	return cSummary, nil
}

// TransportTypes implements API.
func (v *Visor) TransportTypes() ([]string, error) {
	return v.tpM.Networks(), nil
}

// Transports implements API.
func (v *Visor) Transports(types []string, pks []cipher.PubKey, logs bool) ([]*TransportSummary, error) {
	var result []*TransportSummary

	typeIncluded := func(tType network.Type) bool {
		if types != nil {
			for _, ft := range types {
				if string(tType) == ft {
					return true
				}
			}
			return false
		}
		return true
	}
	pkIncluded := func(localPK, remotePK cipher.PubKey) bool {
		if pks != nil {
			for _, fpk := range pks {
				if localPK == fpk || remotePK == fpk {
					return true
				}
			}
			return false
		}
		return true
	}
	v.tpM.WalkTransports(func(tp *transport.ManagedTransport) bool {
		if typeIncluded(tp.Type()) && pkIncluded(v.tpM.Local(), tp.Remote()) {
			result = append(result, newTransportSummary(v.tpM, tp, logs, v.router.SetupIsTrusted(tp.Remote())))
		}
		return true
	})

	return result, nil
}

// Transport implements API.
func (v *Visor) Transport(tid uuid.UUID) (*TransportSummary, error) {
	tp := v.tpM.Transport(tid)
	if tp == nil {
		return nil, ErrNotFound
	}

	return newTransportSummary(v.tpM, tp, true, v.router.SetupIsTrusted(tp.Remote())), nil
}

// AddTransport implements API.
func (v *Visor) AddTransport(remote cipher.PubKey, tpType string, public bool, timeout time.Duration) (*TransportSummary, error) {
	ctx := context.Background()

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Second*20)
		defer cancel()
	}

	v.log.Debugf("Saving transport to %v via %v", remote, tpType)

	tp, err := v.tpM.SaveTransport(ctx, remote, network.Type(tpType), transport.LabelUser)
	if err != nil {
		return nil, err
	}

	v.log.Debugf("Saved transport to %v via %v, label %s", remote, tpType, tp.Entry.Label)

	return newTransportSummary(v.tpM, tp, false, v.router.SetupIsTrusted(tp.Remote())), nil
}

// RemoveTransport implements API.
func (v *Visor) RemoveTransport(tid uuid.UUID) error {
	v.tpM.DeleteTransport(tid)
	return nil
}

// DiscoverTransportsByPK implements API.
func (v *Visor) DiscoverTransportsByPK(pk cipher.PubKey) ([]*transport.EntryWithStatus, error) {
	tpD := v.tpDiscClient()

	entries, err := tpD.GetTransportsByEdge(context.Background(), pk)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// DiscoverTransportByID implements API.
func (v *Visor) DiscoverTransportByID(id uuid.UUID) (*transport.EntryWithStatus, error) {
	tpD := v.tpDiscClient()

	entry, err := tpD.GetTransportByID(context.Background(), id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// RoutingRules implements API.
func (v *Visor) RoutingRules() ([]routing.Rule, error) {
	return v.router.Rules(), nil
}

// RoutingRule implements API.
func (v *Visor) RoutingRule(key routing.RouteID) (routing.Rule, error) {
	return v.router.Rule(key)
}

// SaveRoutingRule implements API.
func (v *Visor) SaveRoutingRule(rule routing.Rule) error {
	return v.router.SaveRule(rule)
}

// RemoveRoutingRule implements API.
func (v *Visor) RemoveRoutingRule(key routing.RouteID) error {
	v.router.DelRules([]routing.RouteID{key})
	return nil
}

// RouteGroups implements API.
func (v *Visor) RouteGroups() ([]RouteGroupInfo, error) {
	var routegroups []RouteGroupInfo

	rules := v.router.Rules()
	for _, rule := range rules {
		if rule.Type() != routing.RuleReverse {
			continue
		}

		fwdRID := rule.NextRouteID()
		rule, err := v.router.Rule(fwdRID)
		if err != nil {
			return nil, err
		}

		routegroups = append(routegroups, RouteGroupInfo{
			ConsumeRule: rule,
			FwdRule:     rule,
		})
	}

	return routegroups, nil
}

// Restart implements API.
func (v *Visor) Restart() error {
	if v.restartCtx == nil {
		return ErrMalformedRestartContext
	}

	return v.restartCtx.Restart()
}

// Exec implements API.
// Exec executes a shell command. It returns combined stdout and stderr output and an error.
func (v *Visor) Exec(command string) ([]byte, error) {
	args := strings.Split(command, " ")
	cmd := exec.Command(args[0], args[1:]...) // nolint: gosec
	return cmd.CombinedOutput()
}

// Update implements API.
// Update updates visor.
// It checks if visor update is available.
// If it is, the method downloads a new visor versions, starts it and kills the current process.
func (v *Visor) Update(updateConfig updater.UpdateConfig) (bool, error) {
	updated, err := v.updater.Update(updateConfig)
	if err != nil {
		v.log.Errorf("Failed to update visor: %v", err)
		return false, err
	}

	return updated, nil
}

// UpdateWithStatus implements API.
// UpdateWithStatus combines results of Update and UpdateStatus.
func (v *Visor) UpdateWithStatus(config updater.UpdateConfig) <-chan StatusMessage {
	ch := make(chan StatusMessage, 512)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				status, err := v.UpdateStatus()
				if err != nil {
					v.log.WithError(err).Errorf("Failed to check update status")
					status = ""
				}

				select {
				case <-ctx.Done():
					return
				default:
					switch status {
					case "", io.EOF.Error():

					default:
						ch <- StatusMessage{
							Text: status,
						}
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	}()

	go func() {
		defer func() {
			cancel()
			close(ch)
		}()

		updated, err := v.Update(config)
		if err != nil {
			ch <- StatusMessage{
				Text:    err.Error(),
				IsError: true,
			}
		} else if updated {
			ch <- StatusMessage{
				Text: "Finished",
			}
		} else {
			ch <- StatusMessage{
				Text: "No update found",
			}
		}
	}()

	return ch
}

// UpdateAvailable implements API.
// UpdateAvailable checks if visor update is available.
func (v *Visor) UpdateAvailable(channel updater.Channel) (*updater.Version, error) {
	version, err := v.updater.UpdateAvailable(channel)
	if err != nil {
		v.log.Errorf("Failed to check if visor update is available: %v", err)
		return nil, err
	}

	return version, nil
}

// UpdateStatus returns status of the current updating operation.
func (v *Visor) UpdateStatus() (string, error) {
	return v.updater.Status(), nil
}

// RuntimeLogs returns visor runtime logs
func (v *Visor) RuntimeLogs() (string, error) {
	var builder strings.Builder
	builder.WriteString("[")
	logs, _ := v.logstore.GetLogs()
	builder.WriteString(strings.Join(logs, ","))
	builder.WriteString("]")
	return builder.String(), nil
}

// SetMinHops sets min_hops routing config of visor
func (v *Visor) SetMinHops(in uint16) error {
	return v.conf.UpdateMinHops(in)
}
