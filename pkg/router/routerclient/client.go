package routerclient

import (
	"context"
	"fmt"
	"io"
	"net/rpc"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/skyenv"
	"github.com/skycoin/skywire/pkg/snet"
)

// RPCName is the RPC gateway object name.
const RPCName = "RPCGateway"

// Client is used to interact with the router's API remotely. The setup node uses this.
type Client struct {
	rpc *rpc.Client
	rPK cipher.PubKey // public key of remote router
	log logrus.FieldLogger
}

// NewClient creates a new Client.
func NewClient(ctx context.Context, dialer snet.Dialer, rPK cipher.PubKey) (*Client, error) {
	s, err := dialer.Dial(ctx, rPK, skyenv.DmsgAwaitSetupPort)
	if err != nil {
		return nil, fmt.Errorf("dial %v@%v: %w", rPK, skyenv.DmsgAwaitSetupPort, err)
	}
	return NewClientFromRaw(s, rPK), nil
}

// NewClientFromRaw creates a new client from a raw connection.
func NewClientFromRaw(conn io.ReadWriteCloser, rPK cipher.PubKey) *Client {
	return &Client{
		rpc: rpc.NewClient(conn),
		rPK: rPK,
		log: logging.MustGetLogger(fmt.Sprintf("router_client:%s", rPK.String())),
	}
}

// Close closes a Client.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.rpc.Close()
}

// AddEdgeRules adds forward and consume rules to router (forward and reverse).
func (c *Client) AddEdgeRules(ctx context.Context, rules routing.EdgeRules) (ok bool, err error) {
	const method = "AddEdgeRules"
	err = c.call(ctx, method, rules, &ok)
	return ok, err
}

// AddEdgeRulesList adds forward and consume rules list to router (forward and reverse).
func (c *Client) AddEdgeRulesList(ctx context.Context, rules routing.EdgeRulesList) (ok bool, err error) {
	const method = "AddEdgeRules" // works as AddEdgeRulesList on newer versions of router
	err = c.call(ctx, method, rules, &ok)
	return ok, err
}

// AddIntermediaryRules adds intermediary rules to router.
func (c *Client) AddIntermediaryRules(ctx context.Context, rules []routing.Rule) (ok bool, err error) {
	const method = "AddIntermediaryRules"
	err = c.call(ctx, method, rules, &ok)
	return ok, err
}

// ReserveIDs reserves n IDs and returns them.
func (c *Client) ReserveIDs(ctx context.Context, n uint8) (rtIDs []routing.RouteID, err error) {
	const method = "ReserveIDs"
	err = c.call(ctx, method, n, &rtIDs)
	return rtIDs, err
}

func (c *Client) call(ctx context.Context, method string, args interface{}, reply interface{}) error {
	call := c.rpc.Go(RPCName+"."+method, args, reply, nil)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-call.Done:
		return call.Error
	}
}
