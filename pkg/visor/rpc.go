package visor

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/google/uuid"

	"github.com/SkycoinProject/skywire-mainnet/pkg/app"
	"github.com/SkycoinProject/skywire-mainnet/pkg/routing"
	"github.com/SkycoinProject/skywire-mainnet/pkg/transport"
)

const (
	// RPCPrefix is the prefix used with all RPC calls.
	RPCPrefix = "app-node"
)

var (
	// ErrInvalidInput occurs when an input is invalid.
	ErrInvalidInput = errors.New("invalid input")

	// ErrNotImplemented occurs when a method is not implemented.
	ErrNotImplemented = errors.New("not implemented")

	// ErrNotFound is returned when a requested resource is not found.
	ErrNotFound = errors.New("not found")

	// ErrMalformedRestartContext is returned when restart context is malformed.
	ErrMalformedRestartContext = errors.New("restart context is malformed")
)

// RPC defines RPC methods for Node.
type RPC struct {
	node *Node
}

/*
	<<< NODE HEALTH >>>
*/

// HealthInfo carries information about visor's external services health represented as http status codes
type HealthInfo struct {
	TransportDiscovery int `json:"transport_discovery"`
	RouteFinder        int `json:"route_finder"`
	SetupNode          int `json:"setup_node"`
}

// Health returns health information about the visor
func (r *RPC) Health(_ *struct{}, out *HealthInfo) error {
	out.TransportDiscovery = http.StatusOK
	out.RouteFinder = http.StatusOK
	out.SetupNode = http.StatusOK

	_, err := r.node.conf.TransportDiscovery()
	if err != nil {
		out.TransportDiscovery = http.StatusNotFound
	}

	if r.node.conf.Routing.RouteFinder == "" {
		out.RouteFinder = http.StatusNotFound
	}

	if len(r.node.conf.Routing.SetupNodes) == 0 {
		out.SetupNode = http.StatusNotFound
	}

	return nil
}

/*
	<<< NODE UPTIME >>>
*/

// Uptime returns for how long the visor has been running in seconds
func (r *RPC) Uptime(_ *struct{}, out *float64) error {
	*out = time.Since(r.node.startedAt).Seconds()
	return nil
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
func (r *RPC) LogsSince(in *AppLogsRequest, out *[]string) error {
	ls, err := app.NewLogStore(filepath.Join(r.node.dir(), in.AppName), in.AppName, "bbolt")
	if err != nil {
		return err
	}

	res, err := ls.LogsSince(in.TimeStamp)
	if err != nil {
		return err
	}

	*out = res
	return nil
}

/*
	<<< NODE SUMMARY >>>
*/

// TransportSummary summarizes a Transport.
type TransportSummary struct {
	ID      uuid.UUID           `json:"id"`
	Local   cipher.PubKey       `json:"local_pk"`
	Remote  cipher.PubKey       `json:"remote_pk"`
	Type    string              `json:"type"`
	Log     *transport.LogEntry `json:"log,omitempty"`
	IsSetup bool                `json:"is_setup"`
}

func newTransportSummary(tm *transport.Manager, tp *transport.ManagedTransport,
	includeLogs bool, isSetup bool) *TransportSummary {

	summary := &TransportSummary{
		ID:      tp.Entry.ID,
		Local:   tm.Local(),
		Remote:  tp.Remote(),
		Type:    tp.Type(),
		IsSetup: isSetup,
	}
	if includeLogs {
		summary.Log = tp.LogEntry
	}
	return summary
}

// Summary provides a summary of a Skywire Visor.
type Summary struct {
	PubKey          cipher.PubKey       `json:"local_pk"`
	NodeVersion     string              `json:"node_version"`
	AppProtoVersion string              `json:"app_protocol_version"`
	Apps            []*AppState         `json:"apps"`
	Transports      []*TransportSummary `json:"transports"`
	RoutesCount     int                 `json:"routes_count"`
}

// Summary provides a summary of the AppNode.
func (r *RPC) Summary(_ *struct{}, out *Summary) error {
	var summaries []*TransportSummary
	r.node.tm.WalkTransports(func(tp *transport.ManagedTransport) bool {
		summaries = append(summaries,
			newTransportSummary(r.node.tm, tp, false, r.node.router.SetupIsTrusted(tp.Remote())))
		return true
	})
	*out = Summary{
		PubKey:          r.node.conf.Node.StaticPubKey,
		NodeVersion:     Version,
		AppProtoVersion: supportedProtocolVersion,
		Apps:            r.node.Apps(),
		Transports:      summaries,
		RoutesCount:     r.node.rt.Count(),
	}
	return nil
}

// Exec executes a given command in cmd and writes its output to out.
func (r *RPC) Exec(cmd *string, out *[]byte) error {
	var err error
	*out, err = r.node.Exec(*cmd)
	return err
}

/*
	<<< APP MANAGEMENT >>>
*/

// Apps returns list of Apps registered on the Node.
func (r *RPC) Apps(_ *struct{}, reply *[]*AppState) error {
	*reply = r.node.Apps()
	return nil
}

// StartApp start App with provided name.
func (r *RPC) StartApp(name *string, _ *struct{}) error {
	return r.node.StartApp(*name)
}

// StopApp stops App with provided name.
func (r *RPC) StopApp(name *string, _ *struct{}) error {
	return r.node.StopApp(*name)
}

// SetAutoStartIn is input for SetAutoStart.
type SetAutoStartIn struct {
	AppName   string
	AutoStart bool
}

// SetAutoStart sets auto-start settings for an app.
func (r *RPC) SetAutoStart(in *SetAutoStartIn, _ *struct{}) error {
	return r.node.SetAutoStart(in.AppName, in.AutoStart)
}

/*
	<<< TRANSPORT MANAGEMENT >>>
*/

// TransportTypes lists all transport types supported by the Node.
func (r *RPC) TransportTypes(_ *struct{}, out *[]string) error {
	*out = r.node.tm.Networks()
	return nil
}

// TransportsIn is input for Transports.
type TransportsIn struct {
	FilterTypes   []string
	FilterPubKeys []cipher.PubKey
	ShowLogs      bool
}

// Transports lists Transports of the Node and provides a summary of each.
func (r *RPC) Transports(in *TransportsIn, out *[]*TransportSummary) error {
	typeIncluded := func(tType string) bool {
		if in.FilterTypes != nil {
			for _, ft := range in.FilterTypes {
				if tType == ft {
					return true
				}
			}
			return false
		}
		return true
	}
	pkIncluded := func(localPK, remotePK cipher.PubKey) bool {
		if in.FilterPubKeys != nil {
			for _, fpk := range in.FilterPubKeys {
				if localPK == fpk || remotePK == fpk {
					return true
				}
			}
			return false
		}
		return true
	}
	r.node.tm.WalkTransports(func(tp *transport.ManagedTransport) bool {
		if typeIncluded(tp.Type()) && pkIncluded(r.node.tm.Local(), tp.Remote()) {
			*out = append(*out, newTransportSummary(r.node.tm, tp, in.ShowLogs, r.node.router.SetupIsTrusted(tp.Remote())))
		}
		return true
	})
	return nil
}

// Transport obtains a Transport Summary of Transport of given Transport ID.
func (r *RPC) Transport(in *uuid.UUID, out *TransportSummary) error {
	tp := r.node.tm.Transport(*in)
	if tp == nil {
		return ErrNotFound
	}
	*out = *newTransportSummary(r.node.tm, tp, true, r.node.router.SetupIsTrusted(tp.Remote()))
	return nil
}

// AddTransportIn is input for AddTransport.
type AddTransportIn struct {
	RemotePK cipher.PubKey
	TpType   string
	Public   bool
	Timeout  time.Duration
}

// AddTransport creates a transport for the node.
func (r *RPC) AddTransport(in *AddTransportIn, out *TransportSummary) error {
	ctx := context.Background()
	if in.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Second*20)
		defer cancel()
	}

	tp, err := r.node.tm.SaveTransport(ctx, in.RemotePK, in.TpType)
	if err != nil {
		return err
	}
	*out = *newTransportSummary(r.node.tm, tp, false, r.node.router.SetupIsTrusted(tp.Remote()))
	return nil
}

// RemoveTransport removes a Transport from the node.
func (r *RPC) RemoveTransport(tid *uuid.UUID, _ *struct{}) error {
	r.node.tm.DeleteTransport(*tid)
	return nil
}

/*
	<<< AVAILABLE TRANSPORTS >>>
*/

// DiscoverTransportsByPK obtains available transports via the transport discovery via given public key.
func (r *RPC) DiscoverTransportsByPK(pk *cipher.PubKey, out *[]*transport.EntryWithStatus) error {
	tpD, err := r.node.conf.TransportDiscovery()
	if err != nil {
		return err
	}
	entries, err := tpD.GetTransportsByEdge(context.Background(), *pk)
	if err != nil {
		return err
	}
	*out = entries
	return nil
}

// DiscoverTransportByID obtains available transports via the transport discovery via a given transport ID.
func (r *RPC) DiscoverTransportByID(id *uuid.UUID, out *transport.EntryWithStatus) error {
	tpD, err := r.node.conf.TransportDiscovery()
	if err != nil {
		return err
	}
	entry, err := tpD.GetTransportByID(context.Background(), *id)
	if err != nil {
		return err
	}
	*out = *entry
	return nil
}

/*
	<<< ROUTES MANAGEMENT >>>
*/

// RoutingRules obtains all routing rules of the RoutingTable.
func (r *RPC) RoutingRules(_ *struct{}, out *[]routing.Rule) error {
	*out = r.node.rt.AllRules()
	return nil
}

// RoutingRule obtains a routing rule of given RouteID.
func (r *RPC) RoutingRule(key *routing.RouteID, rule *routing.Rule) error {
	var err error
	*rule, err = r.node.rt.Rule(*key)
	return err
}

// SaveRoutingRule saves a routing rule.
func (r *RPC) SaveRoutingRule(in *routing.Rule, _ *struct{}) error {
	return r.node.rt.SaveRule(*in)
}

// RemoveRoutingRule removes a RoutingRule based on given RouteID key.
func (r *RPC) RemoveRoutingRule(key *routing.RouteID, _ *struct{}) error {
	r.node.rt.DelRules([]routing.RouteID{*key})
	return nil
}

/*
	<<< LOOPS MANAGEMENT >>>
	>>> TODO(evanlinjin): Implement.
*/

// LoopInfo is a human-understandable representation of a loop.
type LoopInfo struct {
	ConsumeRule routing.Rule
	FwdRule     routing.Rule
}

// Loops retrieves loops via rules of the routing table.
func (r *RPC) Loops(_ *struct{}, out *[]LoopInfo) error {
	var loops []LoopInfo

	rules := r.node.rt.AllRules()
	for _, rule := range rules {
		if rule.Type() != routing.RuleConsume {
			continue
		}

		fwdRID := rule.NextRouteID()
		rule, err := r.node.rt.Rule(fwdRID)
		if err != nil {
			return err
		}
		loops = append(loops, LoopInfo{
			ConsumeRule: rule,
			FwdRule:     rule,
		})
	}

	*out = loops
	return nil
}

/*
	<<< VISOR MANAGEMENT >>>
*/

const exitDelay = 100 * time.Millisecond

// Restart restarts visor.
func (r *RPC) Restart(_ *struct{}, _ *struct{}) (err error) {
	defer func() {
		if err == nil {
			go func() {
				time.Sleep(exitDelay)
				os.Exit(0)
			}()
		}
	}()

	if r.node.restartCtx == nil {
		return ErrMalformedRestartContext
	}

	return r.node.restartCtx.Start()
}
