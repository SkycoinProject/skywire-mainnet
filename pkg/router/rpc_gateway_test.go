package router

import (
	"testing"

	"github.com/skycoin/dmsg/cipher"
	"github.com/stretchr/testify/require"

	"github.com/skycoin/skywire/internal/testhelpers"
	"github.com/skycoin/skywire/pkg/routing"
)

func TestRPCGateway_AddEdgeRules(t *testing.T) {
	srcPK, _ := cipher.GenerateKeyPair()
	dstPK, _ := cipher.GenerateKeyPair()

	var srcPort, dstPort routing.Port = 100, 110

	desc := routing.NewRouteDescriptor(srcPK, dstPK, srcPort, dstPort)

	rules := routing.EdgeRulesList{
		Desc:    desc,
		Forward: []routing.Rule{{0, 0, 0}},
		Reverse: []routing.Rule{{1, 1, 1}},
	}

	t.Run("ok", func(t *testing.T) {
		r := &MockRouter{}
		r.On("IntroduceRules", rules).Return(testhelpers.NoErr)
		r.On("SaveRoutingRules", rules.Forward, rules.Reverse).Return(testhelpers.NoErr)

		gateway := NewRPCGateway(r)

		var ok bool
		err := gateway.AddEdgeRules(rules, &ok)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("fail introducing rules", func(t *testing.T) {
		r := &MockRouter{}
		r.On("IntroduceRules", rules).Return(testhelpers.Err)

		gateway := NewRPCGateway(r)

		var ok bool
		err := gateway.AddEdgeRules(rules, &ok)

		wantErr := routing.Failure{
			Code: routing.FailureAddRules,
			Msg:  testhelpers.Err.Error(),
		}

		require.Equal(t, wantErr, err)
		require.False(t, ok)
	})

	t.Run("fail saving rules", func(t *testing.T) {
		r := &MockRouter{}
		r.On("IntroduceRules", rules).Return(testhelpers.Err)

		gateway := NewRPCGateway(r)

		wantErr := routing.Failure{
			Code: routing.FailureAddRules,
			Msg:  testhelpers.Err.Error(),
		}

		var ok bool
		err := gateway.AddEdgeRules(rules, &ok)
		require.Equal(t, wantErr, err)
		require.False(t, ok)
	})
}

func TestRPCGateway_AddIntermediaryRules(t *testing.T) {
	rule1 := routing.Rule{0, 0, 0}
	rule2 := routing.Rule{1, 1, 1}
	rulesIfc := []interface{}{rule1, rule2}
	rules := []routing.Rule{rule1, rule2}

	t.Run("ok", func(t *testing.T) {
		r := &MockRouter{}
		r.On("SaveRoutingRules", rulesIfc...).Return(testhelpers.NoErr)

		gateway := NewRPCGateway(r)

		var ok bool
		err := gateway.AddIntermediaryRules(rules, &ok)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("fail saving rules", func(t *testing.T) {
		r := &MockRouter{}
		r.On("SaveRoutingRules", rulesIfc...).Return(testhelpers.Err)

		gateway := NewRPCGateway(r)

		wantErr := routing.Failure{
			Code: routing.FailureAddRules,
			Msg:  testhelpers.Err.Error(),
		}

		var ok bool
		err := gateway.AddIntermediaryRules(rules, &ok)
		require.Equal(t, wantErr, err)
		require.False(t, ok)
	})
}

func TestRPCGateway_ReserveIDs(t *testing.T) {
	n := 5
	ids := []routing.RouteID{1, 2, 3, 4, 5}

	t.Run("ok", func(t *testing.T) {
		r := &MockRouter{}
		r.On("ReserveKeys", n).Return(ids, testhelpers.NoErr)

		gateway := NewRPCGateway(r)

		var gotIds []routing.RouteID
		err := gateway.ReserveIDs(uint8(n), &gotIds)
		require.NoError(t, err)
		require.Equal(t, ids, gotIds)
	})

	t.Run("fail reserving keys", func(t *testing.T) {
		r := &MockRouter{}
		r.On("ReserveKeys", n).Return(nil, testhelpers.Err)

		gateway := NewRPCGateway(r)

		wantErr := routing.Failure{
			Code: routing.FailureReserveRtIDs,
			Msg:  testhelpers.Err.Error(),
		}

		var gotIds []routing.RouteID
		err := gateway.ReserveIDs(uint8(n), &gotIds)
		require.Equal(t, wantErr, err)
		require.Nil(t, gotIds)
	})
}
