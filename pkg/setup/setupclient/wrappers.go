package setupclient

import (
	"context"
	"fmt"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/snet"
)

//go:generate mockery -name RouteGroupDialer -case underscore -inpkg

// RouteGroupDialer is an interface for RouteGroup dialers
type RouteGroupDialer interface {
	Dial(
		ctx context.Context,
		log *logging.Logger,
		n *snet.Network,
		setupNodes []cipher.PubKey,
		req routing.BidirectionalRouteList,
	) (routing.EdgeRulesList, error)
}

type setupNodeDialer struct{}

// NewSetupNodeDialer returns a wrapper for (*Client).DialRouteGroupMultiple.
func NewSetupNodeDialer() RouteGroupDialer {
	return new(setupNodeDialer)
}

// Dial dials RouteGroup.
func (d *setupNodeDialer) Dial(
	ctx context.Context,
	log *logging.Logger,
	n *snet.Network,
	setupNodes []cipher.PubKey,
	req routing.BidirectionalRouteList,
) (routing.EdgeRulesList, error) {
	client, err := NewClient(ctx, log, n, setupNodes)
	if err != nil {
		return routing.EdgeRulesList{}, err
	}

	defer func() {
		if err := client.Close(); err != nil {
			log.Warn(err)
		}
	}()

	resp, err := client.DialRouteGroupMultiple(ctx, req)
	if err != nil {
		return routing.EdgeRulesList{}, fmt.Errorf("route setup: %w", err)
	}

	return resp, nil
}
