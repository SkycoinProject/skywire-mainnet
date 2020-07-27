package visor

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/SkycoinProject/dmsg/buildinfo"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/google/uuid"

	"github.com/SkycoinProject/skywire-mainnet/pkg/app/launcher"
	"github.com/SkycoinProject/skywire-mainnet/pkg/routing"
	"github.com/SkycoinProject/skywire-mainnet/pkg/skyenv"
	"github.com/SkycoinProject/skywire-mainnet/pkg/transport"
	"github.com/SkycoinProject/skywire-mainnet/pkg/util/updater"
)

func (v *Visor) Summary() (*Summary, error) {
	var summaries []*TransportSummary
	v.tpM.WalkTransports(func(tp *transport.ManagedTransport) bool {
		summaries = append(summaries,
			newTransportSummary(v.tpM, tp, false, v.router.SetupIsTrusted(tp.Remote())))
		return true
	})

	summary := &Summary{
		PubKey:          v.conf.PK,
		BuildInfo:       buildinfo.Get(),
		AppProtoVersion: supportedProtocolVersion,
		Apps:            v.appL.AppStates(),
		Transports:      summaries,
		RoutesCount:     v.router.RoutesCount(),
	}

	return summary, nil
}

func (v *Visor) Health() (*HealthInfo, error) {
	ctx := context.Background()

	healthInfo := &HealthInfo{
		TransportDiscovery: http.StatusNotFound,
		RouteFinder:        http.StatusNotFound,
		SetupNode:          http.StatusNotFound,
		UptimeTracker:      http.StatusNotFound,
		AddressResolver:    http.StatusNotFound,
	}

	if tdClient := v.tpDiscClient(); tdClient != nil {
		tdStatus, err := tdClient.Health(ctx)
		if err != nil {
			v.log.WithError(err).Warnf("Failed to check transport discovery health")

			healthInfo.TransportDiscovery = http.StatusInternalServerError
		}

		healthInfo.TransportDiscovery = tdStatus
	}

	if rfClient := v.routeFinderClient(); rfClient != nil {
		rfStatus, err := rfClient.Health(ctx)
		if err != nil {
			v.log.WithError(err).Warnf("Failed to check route finder health")

			healthInfo.RouteFinder = http.StatusInternalServerError
		}

		healthInfo.RouteFinder = rfStatus
	}

	// TODO(evanlinjin): This should actually poll the setup nodes services.
	if len(v.conf.Routing.SetupNodes) == 0 {
		healthInfo.SetupNode = http.StatusNotFound
	} else {
		healthInfo.SetupNode = http.StatusOK
	}

	if utClient := v.uptimeTrackerClient(); utClient != nil {
		utStatus, err := utClient.Health(ctx)
		if err != nil {
			v.log.WithError(err).Warnf("Failed to check uptime tracker health")

			healthInfo.UptimeTracker = http.StatusInternalServerError
		}

		healthInfo.UptimeTracker = utStatus
	}

	if arClient := v.addressResolverClient(); arClient != nil {
		arStatus, err := arClient.Health(ctx)
		if err != nil {
			v.log.WithError(err).Warnf("Failed to check address resolver health")

			healthInfo.AddressResolver = http.StatusInternalServerError
		}

		healthInfo.AddressResolver = arStatus
	}

	return healthInfo, nil
}

func (v *Visor) Uptime() (float64, error) {
	return time.Since(v.startedAt).Seconds(), nil
}

func (v *Visor) Apps() ([]*launcher.AppState, error) {
	return v.appL.AppStates(), nil
}

func (v *Visor) StartApp(appName string) error {
	var envs []string
	var err error
	if appName == skyenv.VPNClientName {
		envs, err = makeVPNEnvs(v.conf, v.net)
		if err != nil {
			return err
		}
	}

	return v.appL.StartApp(appName, nil, envs)
}

func (v *Visor) StopApp(appName string) error {
	_, err := v.appL.StopApp(appName)
	return err
}

func (v *Visor) SetAutoStart(appName string, autoStart bool) error {
	if _, ok := v.appL.AppState(appName); !ok {
		return ErrAppProcNotRunning
	}

	v.log.Infof("Saving auto start = %v for app %v to config", autoStart, appName)
	return v.conf.UpdateAppAutostart(v.appL, appName, autoStart)
}

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

	if _, ok := v.procM.ProcByName(appName); ok {
		v.log.Infof("Updated %v password, restarting it", appName)
		return v.appL.RestartApp(appName)
	}

	v.log.Infof("Updated %v password", appName)

	return nil
}

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

	if _, ok := v.procM.ProcByName(appName); ok {
		v.log.Infof("Updated %v PK, restarting it", appName)
		return v.appL.RestartApp(appName)
	}

	v.log.Infof("Updated %v PK", appName)

	return nil
}

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

func (v *Visor) TransportTypes() ([]string, error) {
	return v.tpM.Networks(), nil
}

func (v *Visor) Transports(types []string, pks []cipher.PubKey, logs bool) ([]*TransportSummary, error) {
	var result []*TransportSummary

	typeIncluded := func(tType string) bool {
		if types != nil {
			for _, ft := range types {
				if tType == ft {
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

func (v *Visor) Transport(tid uuid.UUID) (*TransportSummary, error) {
	tp := v.tpM.Transport(tid)
	if tp == nil {
		return nil, ErrNotFound
	}

	return newTransportSummary(v.tpM, tp, true, v.router.SetupIsTrusted(tp.Remote())), nil
}

func (v *Visor) AddTransport(remote cipher.PubKey, tpType string, public bool, timeout time.Duration) (*TransportSummary, error) {
	ctx := context.Background()

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Second*20)
		defer cancel()
	}

	v.log.Debugf("Saving transport to %v via %v", remote, tpType)

	tp, err := v.tpM.SaveTransport(ctx, remote, tpType)
	if err != nil {
		return nil, err
	}

	v.log.Debugf("Saved transport to %v via %v", remote, tpType)

	return newTransportSummary(v.tpM, tp, false, v.router.SetupIsTrusted(tp.Remote())), nil
}

func (v *Visor) RemoveTransport(tid uuid.UUID) error {
	v.tpM.DeleteTransport(tid)
	return nil
}

func (v *Visor) DiscoverTransportsByPK(pk cipher.PubKey) ([]*transport.EntryWithStatus, error) {
	tpD := v.tpDiscClient()

	entries, err := tpD.GetTransportsByEdge(context.Background(), pk)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func (v *Visor) DiscoverTransportByID(id uuid.UUID) (*transport.EntryWithStatus, error) {
	tpD := v.tpDiscClient()

	entry, err := tpD.GetTransportByID(context.Background(), id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (v *Visor) RoutingRules() ([]routing.Rule, error) {
	return v.router.Rules(), nil
}

func (v *Visor) RoutingRule(key routing.RouteID) (routing.Rule, error) {
	return v.router.Rule(key)
}

func (v *Visor) SaveRoutingRule(rule routing.Rule) error {
	return v.router.SaveRule(rule)
}

func (v *Visor) RemoveRoutingRule(key routing.RouteID) error {
	v.router.DelRules([]routing.RouteID{key})
	return nil
}

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

func (v *Visor) Restart() error {
	if v.restartCtx == nil {
		return ErrMalformedRestartContext
	}

	return v.restartCtx.Restart()
}

// Exec executes a shell command. It returns combined stdout and stderr output and an error.
func (v *Visor) Exec(command string) ([]byte, error) {
	args := strings.Split(command, " ")
	cmd := exec.Command(args[0], args[1:]...) // nolint: gosec
	return cmd.CombinedOutput()
}

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

// UpdateAvailable checks if visor update is available.
func (v *Visor) UpdateAvailable(channel updater.Channel) (*updater.Version, error) {
	version, err := v.updater.UpdateAvailable(channel)
	if err != nil {
		v.log.Errorf("Failed to check if visor update is available: %v", err)
		return nil, err
	}

	return version, nil
}