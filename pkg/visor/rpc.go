package visor

import (
	"errors"
	"fmt"
	"net/rpc"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg/cipher"

	"github.com/skycoin/skywire/pkg/app/appserver"
	"github.com/skycoin/skywire/pkg/app/launcher"
	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/transport"
	"github.com/skycoin/skywire/pkg/transport/network"
	"github.com/skycoin/skywire/pkg/util/rpcutil"
	"github.com/skycoin/skywire/pkg/util/updater"
)

const (
	// RPCPrefix is the prefix used with all RPC calls.
	RPCPrefix = "app-visor"
	// HealthTimeout defines timeout for /health endpoint calls done from hypervisor.
	HealthTimeout = 5 * time.Second
	// InnerHealthTimeout defines timeout for /health endpoint calls done from visor.
	// We keep it is less than the `HealthTimeout`, so that the outer call would
	// definitely complete.
	InnerHealthTimeout = 3 * time.Second
)

var (
	// ErrNotImplemented occurs when a method is not implemented.
	ErrNotImplemented = errors.New("not implemented")

	// ErrNotFound is returned when a requested resource is not found.
	ErrNotFound = errors.New("not found")

	// ErrMalformedRestartContext is returned when restart context is malformed.
	ErrMalformedRestartContext = errors.New("restart context is malformed")
)

// RPC defines RPC methods for Visor.
type RPC struct {
	visor API
	log   logrus.FieldLogger
}

func newRPCServer(v *Visor, remoteName string) (*rpc.Server, error) {
	rpcS := rpc.NewServer()
	rpcG := &RPC{
		visor: v,
		log:   v.MasterLogger().PackageLogger("visor_rpc:" + remoteName),
	}

	if err := rpcS.RegisterName(RPCPrefix, rpcG); err != nil {
		return nil, fmt.Errorf("failed to create visor RPC server: %w", err)
	}

	return rpcS, nil
}

/*
	<<< NODE HEALTH >>>
*/

// Health returns health information about the visor
func (r *RPC) Health(_ *struct{}, out *HealthInfo) (err error) {
	defer rpcutil.LogCall(r.log, "Health", nil)(out, &err)

	healthInfo, err := r.visor.Health()
	if healthInfo != nil {
		*out = *healthInfo
	}

	return err
}

/*
	<<< NODE UPTIME >>>
*/

// Uptime returns for how long the visor has been running in seconds
func (r *RPC) Uptime(_ *struct{}, out *float64) (err error) {
	defer rpcutil.LogCall(r.log, "Uptime", nil)(out, &err)

	uptime, err := r.visor.Uptime()
	*out = uptime

	return err
}

/*
	<<< APP LOGS >>>
*/

// AppLogsRequest represents a LogSince method request
type AppLogsRequest struct {
	// TimeStamp should be time.RFC3339Nano formated
	TimeStamp time.Time `json:"time_stamp"`
	// AppName should match the app name in visor config
	AppName string `json:"app_name"`
}

// LogsSince returns all logs from an specific app since the timestamp
func (r *RPC) LogsSince(in *AppLogsRequest, out *[]string) (err error) {
	defer rpcutil.LogCall(r.log, "LogsSince", in)(out, &err)

	logs, err := r.visor.LogsSince(in.TimeStamp, in.AppName)
	*out = logs

	return err
}

/*
	<<< NODE SUMMARY >>>
*/

// TransportSummary summarizes a Transport.
type TransportSummary struct {
	ID      uuid.UUID           `json:"id"`
	Local   cipher.PubKey       `json:"local_pk"`
	Remote  cipher.PubKey       `json:"remote_pk"`
	Type    network.Type        `json:"type"`
	Log     *transport.LogEntry `json:"log,omitempty"`
	IsSetup bool                `json:"is_setup"`
	Label   transport.Label     `json:"label"`
}

func newTransportSummary(tm *transport.Manager, tp *transport.ManagedTransport, includeLogs, isSetup bool) *TransportSummary {
	summary := &TransportSummary{
		ID:      tp.Entry.ID,
		Local:   tm.Local(),
		Remote:  tp.Remote(),
		Type:    tp.Type(),
		IsSetup: isSetup,
		Label:   tp.Entry.Label,
	}
	if includeLogs {
		summary.Log = tp.LogEntry
	}
	return summary
}

// Summary provides an extra summary of the AppNode.
func (r *RPC) Summary(_ *struct{}, out *Summary) (err error) {
	defer rpcutil.LogCall(r.log, "Summary", nil)(out, &err)
	sum, err := r.visor.Summary()
	if err != nil {
		return err
	}
	*out = *sum
	return nil
}

// Overview provides a overview of the AppNode.
func (r *RPC) Overview(_ *struct{}, out *Overview) (err error) {
	defer rpcutil.LogCall(r.log, "Overview", nil)(out, &err)

	overview, err := r.visor.Overview()
	if overview != nil {
		*out = *overview
	}

	return err
}

/*
	<<< APP MANAGEMENT >>>
*/

// SetAppDetailedStatusIn is input for SetAppDetailedStatus.
type SetAppDetailedStatusIn struct {
	AppName string
	Status  string
}

// SetAppDetailedStatus sets app's detailed status.
func (r *RPC) SetAppDetailedStatus(in *SetAppDetailedStatusIn, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "SetAppDetailedStatus", in)(nil, &err)

	return r.visor.SetAppDetailedStatus(in.AppName, in.Status)
}

// Apps returns list of Apps registered on the Visor.
func (r *RPC) Apps(_ *struct{}, reply *[]*launcher.AppState) (err error) {
	defer rpcutil.LogCall(r.log, "Apps", nil)(reply, &err)

	apps, err := r.visor.Apps()
	*reply = apps

	return err
}

// StartApp start App with provided name.
func (r *RPC) StartApp(name *string, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "StartApp", name)(nil, &err)

	return r.visor.StartApp(*name)
}

// StopApp stops App with provided name.
func (r *RPC) StopApp(name *string, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "StopApp", name)(nil, &err)

	return r.visor.StopApp(*name)
}

// RestartApp restarts App with provided name.
func (r *RPC) RestartApp(name *string, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "RestartApp", name)(nil, &err)

	return r.visor.RestartApp(*name)
}

// SetAutoStartIn is input for SetAutoStart.
type SetAutoStartIn struct {
	AppName   string
	AutoStart bool
}

// SetAutoStart sets auto-start settings for an app.
func (r *RPC) SetAutoStart(in *SetAutoStartIn, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "SetAutoStart", in)(nil, &err)

	return r.visor.SetAutoStart(in.AppName, in.AutoStart)
}

// SetAppPasswordIn is input for SetAppPassword.
type SetAppPasswordIn struct {
	AppName  string
	Password string
}

// SetAppPassword sets password for the app.
func (r *RPC) SetAppPassword(in *SetAppPasswordIn, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "SetAppPassword", in)(nil, &err)

	return r.visor.SetAppPassword(in.AppName, in.Password)
}

// SetAppPKIn is input for SetAppPK.
type SetAppPKIn struct {
	AppName string
	PK      cipher.PubKey
}

// SetAppBoolIn is input for SetApp boolean flags
type SetAppBoolIn struct {
	AppName string
	Val     bool
}

// SetAppPK sets PK for the app.
func (r *RPC) SetAppPK(in *SetAppPKIn, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "SetAppPK", in)(nil, &err)

	return r.visor.SetAppPK(in.AppName, in.PK)
}

// SetAppKillswitch sets killswitch flag for the app
func (r *RPC) SetAppKillswitch(in *SetAppBoolIn, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "SetAppKillswitch", in)(nil, &err)

	return r.visor.SetAppKillswitch(in.AppName, in.Val)
}

// SetAppSecure sets secure flag for the app
func (r *RPC) SetAppSecure(in *SetAppBoolIn, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "SetAppSecure", in)(nil, &err)

	return r.visor.SetAppSecure(in.AppName, in.Val)
}

// GetAppStats gets app runtime statistics.
func (r *RPC) GetAppStats(appName *string, out *appserver.AppStats) (err error) {
	defer rpcutil.LogCall(r.log, "GetAppStats", appName)(out, &err)

	stats, err := r.visor.GetAppStats(*appName)
	if err != nil {
		*out = stats
	}

	return err
}

// GetAppConnectionsSummary returns connections stats for the app.
func (r *RPC) GetAppConnectionsSummary(appName *string, out *[]appserver.ConnectionSummary) (err error) {
	defer rpcutil.LogCall(r.log, "GetAppConnectionsSummary", appName)(out, &err)

	summary, err := r.visor.GetAppConnectionsSummary(*appName)
	if summary != nil {
		*out = summary
	}

	return err
}

/*
	<<< TRANSPORT MANAGEMENT >>>
*/

// TransportTypes lists all transport types supported by the Visor.
func (r *RPC) TransportTypes(_ *struct{}, out *[]string) (err error) {
	defer rpcutil.LogCall(r.log, "TransportTypes", nil)(out, &err)

	types, err := r.visor.TransportTypes()
	*out = types

	return err
}

// TransportsIn is input for Transports.
type TransportsIn struct {
	FilterTypes   []string
	FilterPubKeys []cipher.PubKey
	ShowLogs      bool
}

// Transports lists Transports of the Visor and provides a summary of each.
func (r *RPC) Transports(in *TransportsIn, out *[]*TransportSummary) (err error) {
	defer rpcutil.LogCall(r.log, "Transports", in)(out, &err)

	transports, err := r.visor.Transports(in.FilterTypes, in.FilterPubKeys, in.ShowLogs)
	*out = transports

	return err
}

// Transport obtains a Transport Summary of Transport of given Transport ID.
func (r *RPC) Transport(in *uuid.UUID, out *TransportSummary) (err error) {
	defer rpcutil.LogCall(r.log, "Transport", in)(out, &err)

	tp, err := r.visor.Transport(*in)
	if tp != nil {
		*out = *tp
	}

	return err
}

// AddTransportIn is input for AddTransport.
type AddTransportIn struct {
	RemotePK cipher.PubKey
	TpType   string
	Public   bool
	Timeout  time.Duration
}

// AddTransport creates a transport for the visor.
func (r *RPC) AddTransport(in *AddTransportIn, out *TransportSummary) (err error) {
	defer rpcutil.LogCall(r.log, "AddTransport", in)(out, &err)

	tp, err := r.visor.AddTransport(in.RemotePK, in.TpType, in.Public, in.Timeout)
	if tp != nil {
		*out = *tp
	}

	return err
}

// RemoveTransport removes a Transport from the visor.
func (r *RPC) RemoveTransport(tid *uuid.UUID, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "RemoveTransport", tid)(nil, &err)

	return r.visor.RemoveTransport(*tid)
}

/*
	<<< AVAILABLE TRANSPORTS >>>
*/

// DiscoverTransportsByPK obtains available transports via the transport discovery via given public key.
func (r *RPC) DiscoverTransportsByPK(pk *cipher.PubKey, out *[]*transport.EntryWithStatus) (err error) {
	defer rpcutil.LogCall(r.log, "DiscoverTransportsByPK", pk)(out, &err)

	entries, err := r.visor.DiscoverTransportsByPK(*pk)
	*out = entries

	return err
}

// DiscoverTransportByID obtains available transports via the transport discovery via a given transport ID.
func (r *RPC) DiscoverTransportByID(id *uuid.UUID, out *transport.EntryWithStatus) (err error) {
	defer rpcutil.LogCall(r.log, "DiscoverTransportByID", id)(out, &err)

	entry, err := r.visor.DiscoverTransportByID(*id)
	if entry != nil {
		*out = *entry
	}

	return err
}

/*
	<<< ROUTES MANAGEMENT >>>
*/

// RoutingRules obtains all routing rules of the RoutingTable.
func (r *RPC) RoutingRules(_ *struct{}, out *[]routing.Rule) (err error) {
	defer rpcutil.LogCall(r.log, "RoutingRules", nil)(out, &err)

	*out, err = r.visor.RoutingRules()
	return err
}

// RoutingRule obtains a routing rule of given RouteID.
func (r *RPC) RoutingRule(key *routing.RouteID, rule *routing.Rule) (err error) {
	defer rpcutil.LogCall(r.log, "RoutingRule", key)(rule, &err)

	*rule, err = r.visor.RoutingRule(*key)
	return err
}

// SaveRoutingRule saves a routing rule.
func (r *RPC) SaveRoutingRule(in *routing.Rule, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "SaveRoutingRule", in)(nil, &err)

	return r.visor.SaveRoutingRule(*in)
}

// RemoveRoutingRule removes a RoutingRule based on given RouteID key.
func (r *RPC) RemoveRoutingRule(key *routing.RouteID, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "RemoveRoutingRule", key)(nil, &err)

	return r.visor.RemoveRoutingRule(*key)
}

/*
	<<< ROUTEGROUPS MANAGEMENT >>>
	>>> TODO(evanlinjin): Implement.
*/

// RouteGroupInfo is a human-understandable representation of a RouteGroup.
type RouteGroupInfo struct {
	ConsumeRule routing.Rule
	FwdRule     routing.Rule
}

// RouteGroups retrieves routegroups via rules of the routing table.
func (r *RPC) RouteGroups(_ *struct{}, out *[]RouteGroupInfo) (err error) {
	defer rpcutil.LogCall(r.log, "RouteGroups", nil)(out, &err)

	rgs, err := r.visor.RouteGroups()
	*out = rgs

	return err
}

/*
	<<< VISOR MANAGEMENT >>>
*/

// Restart restarts visor.
func (r *RPC) Restart(_ *struct{}, _ *struct{}) (err error) {
	// @evanlinjin: do not defer this log statement, as the underlying visor.Logger will get closed.
	rpcutil.LogCall(r.log, "Restart", nil)(nil, nil)

	return r.visor.Restart()
}

// Exec executes a given command in cmd and writes its output to out.
func (r *RPC) Exec(cmd *string, out *[]byte) (err error) {
	defer rpcutil.LogCall(r.log, "Exec", cmd)(out, &err)

	*out, err = r.visor.Exec(*cmd)
	return err
}

// Update updates visor.
func (r *RPC) Update(updateConfig *updater.UpdateConfig, updated *bool) (err error) {
	defer rpcutil.LogCall(r.log, "Update", updateConfig)(updated, &err)

	config := updater.UpdateConfig{}

	if updateConfig != nil {
		config = *updateConfig
	}

	*updated, err = r.visor.Update(config)
	return
}

// UpdateAvailable checks if visor update is available.
func (r *RPC) UpdateAvailable(channel *updater.Channel, version *updater.Version) (err error) {
	defer rpcutil.LogCall(r.log, "UpdateAvailable", channel)(version, &err)

	if channel == nil {
		return updater.ErrUnknownChannel
	}

	v, err := r.visor.UpdateAvailable(*channel)
	if err != nil {
		return err
	}

	if v == nil {
		return nil
	}

	*version = *v
	return nil
}

// UpdateStatus returns visor update status.
func (r *RPC) UpdateStatus(_ *struct{}, status *string) (err error) {
	*status, err = r.visor.UpdateStatus()
	return
}

// RuntimeLogs returns visor runtime logs
func (r *RPC) RuntimeLogs(_ *struct{}, logs *string) (err error) {
	*logs, err = r.visor.RuntimeLogs()
	return
}

// SetMinHops sets min_hops from visor's routing config
func (r *RPC) SetMinHops(n *uint16, _ *struct{}) (err error) {
	defer rpcutil.LogCall(r.log, "SetMinHops", *n)
	err = r.visor.SetMinHops(*n)
	return
}
