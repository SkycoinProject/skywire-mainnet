package routerclient

import (
	"context"
	"net"
	"net/rpc"
	"testing"

	"github.com/skycoin/dmsg/cipher"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"

	"github.com/skycoin/skywire/internal/testhelpers"
	"github.com/skycoin/skywire/pkg/router"
	"github.com/skycoin/skywire/pkg/routing"
)

func TestClient_Close(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		r := &router.MockRouter{}
		_, _, cleanup := prepRPCServerAndClient(t, r)
		cleanup()
	})

	t.Run("ok - no panic on nil client", func(t *testing.T) {
		var cl *Client
		err := cl.Close()
		require.NoError(t, err)
	})
}

func TestClient_AddEdgeRules(t *testing.T) {
	srcPK, _ := cipher.GenerateKeyPair()
	dstPK, _ := cipher.GenerateKeyPair()

	var srcPort, dstPort routing.Port = 100, 110

	desc := routing.NewRouteDescriptor(srcPK, dstPK, srcPort, dstPort)

	rules := routing.EdgeRulesList{
		Desc:    desc,
		Forward: []routing.Rule{{0, 0, 0}},
		Reverse: []routing.Rule{{1, 1, 1}},
	}

	r := &router.MockRouter{}
	r.On("IntroduceRules", rules).Return(testhelpers.NoErr)
	r.On("SaveRoutingRules", rules.Forward, rules.Reverse).Return(testhelpers.NoErr)

	_, cl, cleanup := prepRPCServerAndClient(t, r)
	defer cleanup()

	ok, err := cl.AddEdgeRulesList(context.Background(), rules)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestClient_AddIntermediaryRules(t *testing.T) {
	rule1 := routing.Rule{0, 0, 0}
	rule2 := routing.Rule{1, 1, 1}
	rulesIfc := []interface{}{rule1, rule2}
	rules := []routing.Rule{rule1, rule2}

	r := &router.MockRouter{}
	r.On("SaveRoutingRules", rulesIfc...).Return(testhelpers.NoErr)

	_, cl, cleanup := prepRPCServerAndClient(t, r)
	defer cleanup()

	ok, err := cl.AddIntermediaryRules(context.Background(), rules)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestClient_ReserveIDs(t *testing.T) {
	n := uint8(5)
	ids := []routing.RouteID{1, 2, 3, 4, 5}

	r := &router.MockRouter{}
	r.On("ReserveKeys", int(n)).Return(ids, testhelpers.NoErr)

	_, cl, cleanup := prepRPCServerAndClient(t, r)
	defer cleanup()

	gotIDs, err := cl.ReserveIDs(context.Background(), n)
	require.NoError(t, err)
	require.Equal(t, ids, gotIDs)
}

// nolint:unparam
func prepRPCServerAndClient(t *testing.T, r router.Router) (s *rpc.Server, cl *Client, cleanup func()) {
	l, err := nettest.NewLocalListener("tcp")
	require.NoError(t, err)

	s = rpc.NewServer()
	require.NoError(t, s.Register(router.NewRPCGateway(r)))
	go s.Accept(l)

	conn, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)

	cl = &Client{
		rpc: rpc.NewClient(conn),
	}

	cleanup = func() {
		err := cl.Close()
		require.NoError(t, err)

		err = l.Close()
		require.NoError(t, err)
	}

	return s, cl, cleanup
}
