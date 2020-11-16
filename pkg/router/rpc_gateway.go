package router

import (
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/pkg/routing"
)

// RPCGateway is a RPC interface for router.
type RPCGateway struct {
	logger *logging.Logger
	router Router
}

// NewRPCGateway creates a new RPCGateway.
func NewRPCGateway(router Router) *RPCGateway {
	return &RPCGateway{
		logger: logging.MustGetLogger("router-gateway"),
		router: router,
	}
}

// AddEdgeRules adds edge rules.
func (r *RPCGateway) AddEdgeRules(rules routing.EdgeRulesList, ok *bool) error {
	if err := r.router.IntroduceRules(rules); err != nil {
		*ok = false

		r.logger.WithError(err).Warnf("Request completed with error.")

		return routing.Failure{Code: routing.FailureAddRules, Msg: err.Error()}
	}

	*ok = true

	return nil
}

// AddIntermediaryRules adds intermediary rules.
func (r *RPCGateway) AddIntermediaryRules(rules []routing.Rule, ok *bool) error {
	if err := r.router.SaveRoutingRules(rules...); err != nil {
		*ok = false

		r.logger.WithError(err).Warnf("Request completed with error.")

		return routing.Failure{Code: routing.FailureAddRules, Msg: err.Error()}
	}

	*ok = true

	return nil
}

// ReserveIDs reserves route IDs.
func (r *RPCGateway) ReserveIDs(n uint8, routeIDs *[]routing.RouteID) error {
	ids, err := r.router.ReserveKeys(int(n))
	if err != nil {
		r.logger.WithError(err).Warnf("Request completed with error.")
		return routing.Failure{Code: routing.FailureReserveRtIDs, Msg: err.Error()}
	}

	*routeIDs = ids

	return nil
}
