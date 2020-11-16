package router

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/skycoin/skywire/internal/testhelpers"
	"github.com/skycoin/skywire/pkg/routefinder/rfclient"
	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/setup/setupclient"
	"github.com/skycoin/skywire/pkg/snet"
	"github.com/skycoin/skywire/pkg/snet/snettest"
	"github.com/skycoin/skywire/pkg/transport"
)

func TestMain(m *testing.M) {
	loggingLevel, ok := os.LookupEnv("TEST_LOGGING_LEVEL")
	if ok {
		lvl, err := logging.LevelFromString(loggingLevel)
		if err != nil {
			log.Fatal(err)
		}

		logging.SetLevel(lvl)
	} else {
		logging.SetLevel(logrus.TraceLevel)
	}

	os.Exit(m.Run())
}

// Test ensures that we can establish connection between 2 routers. 1st router dials
// the 2nd one, 2nd one accepts. We get 2 noise-wrapped route groups and check that
// these route groups correctly communicate with each other.
func Test_router_NoiseRouteGroups(t *testing.T) {
	// We're doing 2 key pairs for 2 communicating routers.
	keys := snettest.GenKeyPairs(2)

	desc := routing.NewRouteDescriptor(keys[0].PK, keys[1].PK, 1, 1)

	forwardHops := []routing.Hop{
		{From: keys[0].PK, To: keys[1].PK, TpID: transport.MakeTransportID(keys[0].PK, keys[1].PK, dmsg.Type)},
	}

	reverseHops := []routing.Hop{
		{From: keys[1].PK, To: keys[0].PK, TpID: transport.MakeTransportID(keys[1].PK, keys[0].PK, dmsg.Type)},
	}

	// Route that will be established
	route := routing.BidirectionalRouteList{
		Desc:      desc,
		KeepAlive: DefaultRouteKeepAlive,
		Forward:   [][]routing.Hop{forwardHops},
		Reverse:   [][]routing.Hop{reverseHops},
	}

	// Create test env
	nEnv := snettest.NewEnv(t, keys, []string{dmsg.Type})
	defer nEnv.Teardown()

	tpD := transport.NewDiscoveryMock()

	// Prepare transports
	m0, m1, _, _, err := transport.CreateTransportPair(tpD, keys[:2], nEnv, dmsg.Type)
	require.NoError(t, err)

	forward := [2]cipher.PubKey{keys[0].PK, keys[1].PK}
	backward := [2]cipher.PubKey{keys[1].PK, keys[0].PK}

	// Paths to be returned from route finder
	rfPaths := make(map[routing.PathEdges][][]routing.Hop)
	rfPaths[forward] = append(rfPaths[forward], forwardHops)
	rfPaths[backward] = append(rfPaths[backward], reverseHops)

	rfCl := &rfclient.MockClient{}
	rfCl.On("FindRoutes", mock.Anything, []routing.PathEdges{forward, backward},
		&rfclient.RouteOptions{MinHops: minHops, MaxHops: maxHops}).Return(rfPaths, testhelpers.NoErr)

	r0Logger := logging.MustGetLogger(fmt.Sprintf("router_%d", 0))

	fwdRt, revRt := route.ForwardAndReverse()
	srcPK := route.Desc.SrcPK()
	dstPK := route.Desc.DstPK()

	fwdRules0 := routing.ForwardRule(route.KeepAlive, 1, 2, forwardHops[0].TpID, srcPK, dstPK, 1, 1)
	revRules0 := routing.ConsumeRule(route.KeepAlive, 3, srcPK, dstPK, 1, 1)

	// Edge rules to be returned from route group dialer
	initEdge := routing.EdgeRulesList{Desc: revRt[0].Desc, Forward: []routing.Rule{fwdRules0}, Reverse: []routing.Rule{revRules0}}

	setupCl0 := &setupclient.MockRouteGroupDialer{}
	setupCl0.On("Dial", mock.Anything, r0Logger, nEnv.Nets[0], mock.Anything, route).
		Return(initEdge, testhelpers.NoErr)

	r0Conf := &Config{
		Logger:           r0Logger,
		PubKey:           keys[0].PK,
		SecKey:           keys[0].SK,
		TransportManager: m0,
		RouteFinder:      rfCl,
		RouteGroupDialer: setupCl0,
	}

	// Create routers
	r0Ifc, err := New(nEnv.Nets[0], r0Conf)
	require.NoError(t, err)

	r0, ok := r0Ifc.(*router)
	require.True(t, ok)

	r1Conf := &Config{
		Logger:           logging.MustGetLogger(fmt.Sprintf("router_%d", 1)),
		PubKey:           keys[1].PK,
		SecKey:           keys[1].SK,
		TransportManager: m1,
	}

	r1Ifc, err := New(nEnv.Nets[1], r1Conf)
	require.NoError(t, err)

	r1, ok := r1Ifc.(*router)
	require.True(t, ok)

	ctx := context.Background()

	nrg1IfcCh := make(chan net.Conn)
	acceptErrCh := make(chan error)
	go func() {
		nrg1Ifc, err := r1.AcceptRoutes(ctx)
		acceptErrCh <- err
		nrg1IfcCh <- nrg1Ifc
		close(acceptErrCh)
		close(nrg1IfcCh)
	}()

	dialErrCh := make(chan error)
	nrg0IfcCh := make(chan net.Conn)
	go func() {
		nrg0Ifc, err := r0.DialRoutes(context.Background(), r1.conf.PubKey, 1, 1, nil)
		dialErrCh <- err
		nrg0IfcCh <- nrg0Ifc
		close(dialErrCh)
		close(nrg0IfcCh)
	}()

	fwdRules1 := routing.ForwardRule(route.KeepAlive, 4, 3, reverseHops[0].TpID, dstPK, srcPK, 1, 1)
	revRules1 := routing.ConsumeRule(route.KeepAlive, 2, dstPK, srcPK, 1, 1)

	// This edge is returned by the setup node to accepting router
	respEdge := routing.EdgeRulesList{Desc: fwdRt[0].Desc, Forward: []routing.Rule{fwdRules1}, Reverse: []routing.Rule{revRules1}}

	// Unblock AcceptRoutes, imitates setup node request with EdgeRulesList
	r1.accept <- respEdge

	// At some point raw route group gets into `rgsRaw` and waits for
	// handshake packets. we're waiting for this moment in the cycle
	// to start passing packets from the transport to route group
	for {
		r0.mx.Lock()
		if _, ok := r0.rgsRaw[initEdge.Desc]; ok {
			rg := r0.rgsRaw[initEdge.Desc]
			go pushPackets(ctx, m0, rg)
			r0.mx.Unlock()
			break
		}
		r0.mx.Unlock()
	}

	for {
		r1.mx.Lock()
		if _, ok := r1.rgsRaw[respEdge.Desc]; ok {
			rg := r1.rgsRaw[respEdge.Desc]
			go pushPackets(ctx, m1, rg)
			r1.mx.Unlock()
			break
		}
		r1.mx.Unlock()
	}

	require.NoError(t, <-acceptErrCh)
	require.NoError(t, <-dialErrCh)

	nrg0Ifc := <-nrg0IfcCh
	require.NotNil(t, nrg0Ifc)
	nrg1Ifc := <-nrg1IfcCh
	require.NotNil(t, nrg1Ifc)

	nrg0, ok := nrg0Ifc.(*NoiseRouteGroup)
	require.True(t, ok)
	require.NotNil(t, nrg0)

	nrg1, ok := nrg1Ifc.(*NoiseRouteGroup)
	require.True(t, ok)
	require.NotNil(t, nrg1)

	data := []byte("Hello there!")
	n, err := nrg0.Write(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	received := make([]byte, 1024)
	n, err = nrg1.Read(received)
	require.NoError(t, err)
	require.Equal(t, len(data), n)
	require.Equal(t, data, received[:n])

	require.True(t, nrg0.IsAlive())
	require.True(t, nrg1.IsAlive())

	err = nrg0.Close()
	require.NoError(t, err)

	require.False(t, nrg0.IsAlive())
	require.False(t, nrg1.IsAlive())

	require.True(t, nrg1.rg.isRemoteClosed())
	err = nrg1.Close()
	require.NoError(t, err)
	require.False(t, nrg1.IsAlive())
}

func TestRouter_Serve(t *testing.T) {
	// We are generating two key pairs - one for the a `Router`, the other to send packets to `Router`.
	keys := snettest.GenKeyPairs(2)

	// create test env
	nEnv := snettest.NewEnv(t, keys, []string{dmsg.Type})
	defer nEnv.Teardown()

	rEnv := NewTestEnv(t, nEnv.Nets)
	defer rEnv.Teardown()

	// Create routers
	r0Ifc, err := New(nEnv.Nets[0], rEnv.GenRouterConfig(0))
	require.NoError(t, err)

	r0, ok := r0Ifc.(*router)
	require.True(t, ok)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, r0.tm.Close())
	require.NoError(t, r0.Serve(ctx))
}

const ruleKeepAlive = 1 * time.Hour

// Ensure that received packets are handled properly in `(*Router).handleTransportPacket()`.
func TestRouter_handleTransportPacket(t *testing.T) {
	// We are generating two key pairs - one for the a `Router`, the other to send packets to `Router`.
	keys := snettest.GenKeyPairs(2)

	pk1 := keys[0].PK
	pk2 := keys[1].PK

	// create test env
	nEnv := snettest.NewEnv(t, keys, []string{dmsg.Type})
	defer nEnv.Teardown()

	rEnv := NewTestEnv(t, nEnv.Nets)
	defer rEnv.Teardown()

	// Create routers
	r0Ifc, err := New(nEnv.Nets[0], rEnv.GenRouterConfig(0))
	require.NoError(t, err)

	r0, ok := r0Ifc.(*router)
	require.True(t, ok)

	r1Ifc, err := New(nEnv.Nets[1], rEnv.GenRouterConfig(1))
	require.NoError(t, err)

	r1, ok := r1Ifc.(*router)
	require.True(t, ok)

	defer func() {
		require.NoError(t, r0.Close())
		require.NoError(t, r1.Close())
	}()

	// Create dmsg transport between two `snet.Network` entities.
	tp1, err := rEnv.TpMngrs[1].SaveTransport(context.TODO(), pk1, dmsg.Type)
	require.NoError(t, err)

	testHandlePackets(t, r0, r1, tp1, pk1, pk2)
}

func testHandlePackets(t *testing.T, r0, r1 *router, tp1 *transport.ManagedTransport, pk1, pk2 cipher.PubKey) {
	var wg sync.WaitGroup

	wg.Add(1)
	t.Run("handlePacket_fwdRule", func(t *testing.T) {
		defer wg.Done()

		testForwardRule(t, r0, r1, tp1, pk1, pk2)
	})
	wg.Wait()

	wg.Add(1)
	t.Run("handlePacket_intFwdRule", func(t *testing.T) {
		defer wg.Done()

		testIntermediaryForwardRule(t, r0, r1, tp1)
	})
	wg.Wait()

	wg.Add(1)
	t.Run("handlePacket_cnsmRule", func(t *testing.T) {
		defer wg.Done()

		testConsumeRule(t, r0, r1, tp1, pk1, pk2)
	})
	wg.Wait()

	wg.Add(1)
	t.Run("handlePacket_close_initiator", func(t *testing.T) {
		defer wg.Done()

		testClosePacketInitiator(t, r0, r1, pk1, pk2, tp1)
	})
	wg.Wait()

	wg.Add(1)
	t.Run("handlePacket_close_remote", func(t *testing.T) {
		defer wg.Done()

		testClosePacketRemote(t, r0, r1, pk1, pk2, tp1)
	})
	wg.Wait()

	wg.Add(1)
	t.Run("handlePacket_keepalive", func(t *testing.T) {
		defer wg.Done()

		testKeepAlivePacket(t, r0, r1, pk1, pk2)
	})
	wg.Wait()
}

func testKeepAlivePacket(t *testing.T, r0, r1 *router, pk1, pk2 cipher.PubKey) {
	defer clearRouterRules(r0, r1)
	defer clearRouteGroups(r0, r1)

	rtIDs, err := r0.ReserveKeys(1)
	require.NoError(t, err)

	rtID := rtIDs[0]

	cnsmRule := routing.ConsumeRule(100*time.Millisecond, rtID, pk2, pk1, 0, 0)
	err = r0.rt.SaveRule(cnsmRule)
	require.NoError(t, err)
	require.Len(t, r0.rt.AllRules(), 1)

	time.Sleep(50 * time.Millisecond)

	packet := routing.MakeKeepAlivePacket(rtIDs[0])
	require.NoError(t, r0.handleTransportPacket(context.TODO(), packet))

	require.Len(t, r0.rt.AllRules(), 1)
	time.Sleep(40 * time.Millisecond)
	require.Len(t, r0.rt.AllRules(), 1)

	time.Sleep(110 * time.Millisecond)
	require.Len(t, r0.rt.AllRules(), 0)
}

func testClosePacketRemote(t *testing.T, r0, r1 *router, pk1, pk2 cipher.PubKey, tp1 *transport.ManagedTransport) {
	defer clearRouterRules(r0, r1)
	defer clearRouteGroups(r0, r1)

	// reserve FWD IDs for r0.
	intFwdID, err := r0.ReserveKeys(1)
	require.NoError(t, err)

	// reserve FWD and CNSM IDs for r1.
	r1RtIDs, err := r1.ReserveKeys(2)
	require.NoError(t, err)

	intFwdRule := routing.IntermediaryForwardRule(1*time.Hour, intFwdID[0], r1RtIDs[1], tp1.Entry.ID)
	err = r0.rt.SaveRule(intFwdRule)
	require.NoError(t, err)

	routeID := routing.RouteID(7)
	fwdRule := routing.ForwardRule(ruleKeepAlive, r1RtIDs[0], routeID, tp1.Entry.ID, pk1, pk2, 0, 0)
	cnsmRule := routing.ConsumeRule(ruleKeepAlive, r1RtIDs[1], pk2, pk1, 0, 0)

	err = r1.rt.SaveRule(fwdRule)
	require.NoError(t, err)

	err = r1.rt.SaveRule(cnsmRule)
	require.NoError(t, err)

	fwdRtDesc := fwdRule.RouteDescriptor()

	rules := routing.EdgeRulesList{
		Desc:    fwdRtDesc.Invert(),
		Forward: []routing.Rule{fwdRule},
		Reverse: []routing.Rule{cnsmRule},
	}

	rg1 := NewRouteGroup(DefaultRouteGroupConfig(), r1.rt, rules.Desc)
	rg1.appendRules(rules.Forward[0], rules.Reverse[0], r1.tm.Transport(rules.Forward[0].NextTransportID()))

	nrg1 := &NoiseRouteGroup{rg: rg1}
	r1.rgsNs[rg1.desc] = nrg1

	packet := routing.MakeClosePacket(intFwdID[0], routing.CloseRequested)
	err = r0.handleTransportPacket(context.TODO(), packet)
	require.NoError(t, err)

	recvPacket, err := r1.tm.ReadPacket()
	require.NoError(t, err)
	require.Equal(t, packet.Size(), recvPacket.Size())
	require.Equal(t, packet.Payload(), recvPacket.Payload())
	require.Equal(t, packet.Type(), recvPacket.Type())
	require.Equal(t, r1RtIDs[1], recvPacket.RouteID())

	err = r1.handleTransportPacket(context.TODO(), recvPacket)
	require.NoError(t, err)

	require.True(t, nrg1.rg.isRemoteClosed())
	require.False(t, nrg1.isClosed())
	require.Len(t, r1.rgsNs, 0)
	require.Len(t, r0.rt.AllRules(), 0)
	require.Len(t, r1.rt.AllRules(), 0)
}

func testClosePacketInitiator(t *testing.T, r0, r1 *router, pk1, pk2 cipher.PubKey, tp1 *transport.ManagedTransport) {
	defer clearRouterRules(r0, r1)
	defer clearRouteGroups(r0, r1)

	// reserve FWD IDs for r0.
	intFwdID, err := r0.ReserveKeys(1)
	require.NoError(t, err)

	// reserve FWD and CNSM IDs for r1.
	r1RtIDs, err := r1.ReserveKeys(2)
	require.NoError(t, err)

	intFwdRule := routing.IntermediaryForwardRule(1*time.Hour, intFwdID[0], r1RtIDs[1], tp1.Entry.ID)
	err = r0.rt.SaveRule(intFwdRule)
	require.NoError(t, err)

	routeID := routing.RouteID(7)
	fwdRule := routing.ForwardRule(ruleKeepAlive, r1RtIDs[0], routeID, tp1.Entry.ID, pk1, pk2, 0, 0)
	cnsmRule := routing.ConsumeRule(ruleKeepAlive, r1RtIDs[1], pk2, pk1, 0, 0)

	err = r1.rt.SaveRule(fwdRule)
	require.NoError(t, err)

	err = r1.rt.SaveRule(cnsmRule)
	require.NoError(t, err)

	fwdRtDesc := fwdRule.RouteDescriptor()

	rules := routing.EdgeRulesList{
		Desc:    fwdRtDesc.Invert(),
		Forward: []routing.Rule{fwdRule},
		Reverse: []routing.Rule{cnsmRule},
	}

	rg1 := NewRouteGroup(DefaultRouteGroupConfig(), r1.rt, rules.Desc)
	rg1.appendRules(rules.Forward[0], rules.Reverse[0], r1.tm.Transport(rules.Forward[0].NextTransportID()))

	nrg1 := &NoiseRouteGroup{rg: rg1}
	r1.rgsNs[rg1.desc] = nrg1

	packet := routing.MakeClosePacket(intFwdID[0], routing.CloseRequested)
	err = r0.handleTransportPacket(context.TODO(), packet)
	require.NoError(t, err)

	recvPacket, err := r1.tm.ReadPacket()
	require.NoError(t, err)
	require.Equal(t, packet.Size(), recvPacket.Size())
	require.Equal(t, packet.Payload(), recvPacket.Payload())
	require.Equal(t, packet.Type(), recvPacket.Type())
	require.Equal(t, r1RtIDs[1], recvPacket.RouteID())

	rg1.closeDone.Add(1)
	rg1.closeInitiated = 1

	err = r1.handleTransportPacket(context.TODO(), recvPacket)
	require.NoError(t, err)

	require.Len(t, r1.rgsNs, 0)
	require.Len(t, r0.rt.AllRules(), 0)
	// since this is the close initiator but the close routine wasn't called,
	// forward rule is left
	require.Len(t, r1.rt.AllRules(), 1)
}

// TEST: Ensure handleTransportPacket does as expected.
// After setting a rule in r0, r0 should forward a packet to r1 (as specified in the given rule)
// when r0.handleTransportPacket() is called.
func testForwardRule(t *testing.T, r0, r1 *router, tp1 *transport.ManagedTransport, pk1, pk2 cipher.PubKey) {
	defer clearRouterRules(r0, r1)
	defer clearRouteGroups(r0, r1)

	// Add a FWD rule for r0.
	fwdRtID, err := r0.ReserveKeys(1)
	require.NoError(t, err)

	routeID := routing.RouteID(1)
	fwdRule := routing.ForwardRule(ruleKeepAlive, fwdRtID[0], routeID, tp1.Entry.ID, pk1, pk2, 0, 0)
	err = r0.rt.SaveRule(fwdRule)
	require.NoError(t, err)

	rg0 := NewRouteGroup(DefaultRouteGroupConfig(), r0.rt, fwdRule.RouteDescriptor())
	rg0.appendRules(fwdRule, nil, r0.tm.Transport(fwdRule.NextTransportID()))

	nrg0 := &NoiseRouteGroup{rg: rg0}
	r0.rgsNs[rg0.desc] = nrg0

	// Call handleTransportPacket for r0 (this should in turn, use the rule we added).
	packet, err := routing.MakeDataPacket(fwdRtID[0], []byte("This is a test!"))
	require.NoError(t, err)

	require.NoError(t, r0.handleTransportPacket(context.TODO(), packet))

	// r1 should receive the packet handled by r0.
	recvPacket, err := r1.tm.ReadPacket()
	assert.NoError(t, err)
	assert.Equal(t, packet.Size(), recvPacket.Size())
	assert.Equal(t, packet.Payload(), recvPacket.Payload())
	assert.Equal(t, routeID, recvPacket.RouteID())
}

func testIntermediaryForwardRule(t *testing.T, r0, r1 *router, tp1 *transport.ManagedTransport) {
	defer clearRouterRules(r0, r1)
	defer clearRouteGroups(r0, r1)

	// Add a FWD rule for r0.
	fwdRtID, err := r0.ReserveKeys(1)
	require.NoError(t, err)

	fwdRule := routing.IntermediaryForwardRule(ruleKeepAlive, fwdRtID[0], routing.RouteID(5), tp1.Entry.ID)
	err = r0.rt.SaveRule(fwdRule)
	require.NoError(t, err)

	// Call handleTransportPacket for r0 (this should in turn, use the rule we added).
	packet, err := routing.MakeDataPacket(fwdRtID[0], []byte("This is a test!"))
	require.NoError(t, err)

	require.NoError(t, r0.handleTransportPacket(context.TODO(), packet))

	// r1 should receive the packet handled by r0.
	recvPacket, err := r1.tm.ReadPacket()
	assert.NoError(t, err)
	assert.Equal(t, packet.Size(), recvPacket.Size())
	assert.Equal(t, packet.Payload(), recvPacket.Payload())
	assert.Equal(t, routing.RouteID(5), recvPacket.RouteID())
}

func testConsumeRule(t *testing.T, r0, r1 *router, tp1 *transport.ManagedTransport, pk1, pk2 cipher.PubKey) {
	defer clearRouterRules(r0, r1)
	defer clearRouteGroups(r0, r1)

	// one for consume rule and one for reverse forward rule
	dstRtIDs, err := r1.ReserveKeys(2)
	require.NoError(t, err)

	intFwdRtID, err := r0.ReserveKeys(1)
	require.NoError(t, err)

	intFwdRule := routing.IntermediaryForwardRule(ruleKeepAlive, intFwdRtID[0], dstRtIDs[1], tp1.Entry.ID)
	err = r0.rt.SaveRule(intFwdRule)
	require.NoError(t, err)

	routeID := routing.RouteID(7)
	fwdRule := routing.ForwardRule(ruleKeepAlive, dstRtIDs[0], routeID, tp1.Entry.ID, pk1, pk2, 0, 0)
	cnsmRule := routing.ConsumeRule(ruleKeepAlive, dstRtIDs[1], pk2, pk1, 0, 0)

	err = r1.rt.SaveRule(fwdRule)
	require.NoError(t, err)

	err = r1.rt.SaveRule(cnsmRule)
	require.NoError(t, err)

	fwdRtDesc := fwdRule.RouteDescriptor()

	rules := routing.EdgeRulesList{
		Desc:    fwdRtDesc.Invert(),
		Forward: []routing.Rule{fwdRule},
		Reverse: []routing.Rule{cnsmRule},
	}

	rg1 := NewRouteGroup(DefaultRouteGroupConfig(), r1.rt, rules.Desc)
	rg1.appendRules(rules.Forward[0], rules.Reverse[0], r1.tm.Transport(rules.Forward[0].NextTransportID()))

	nrg1 := &NoiseRouteGroup{rg: rg1}
	r1.rgsNs[rg1.desc] = nrg1

	packet, err := routing.MakeDataPacket(intFwdRtID[0], []byte("test intermediary forward"))
	require.NoError(t, err)

	require.NoError(t, r0.handleTransportPacket(context.TODO(), packet))

	recvPacket, err := r1.tm.ReadPacket()
	assert.NoError(t, err)
	assert.Equal(t, packet.Size(), recvPacket.Size())
	assert.Equal(t, packet.Payload(), recvPacket.Payload())
	assert.Equal(t, dstRtIDs[1], recvPacket.RouteID())

	consumeMsg := []byte("test_consume")
	packet, err = routing.MakeDataPacket(dstRtIDs[1], consumeMsg)
	require.NoError(t, err)

	require.NoError(t, r1.handleTransportPacket(context.TODO(), packet))

	nrg, ok := r1.noiseRouteGroup(fwdRtDesc.Invert())
	require.True(t, ok)
	require.NotNil(t, nrg)

	data := <-nrg.rg.readCh
	require.Equal(t, consumeMsg, data)
}

func TestRouter_Rules(t *testing.T) {
	pk, sk := cipher.GenerateKeyPair()

	env := snettest.NewEnv(t, []snettest.KeyPair{{PK: pk, SK: sk}}, []string{dmsg.Type})
	defer env.Teardown()

	rt := routing.NewTable()

	// We are generating two key pairs - one for the a `Router`, the other to send packets to `Router`.
	keys := snettest.GenKeyPairs(2)

	// create test env
	nEnv := snettest.NewEnv(t, keys, []string{dmsg.Type})
	defer nEnv.Teardown()

	rEnv := NewTestEnv(t, nEnv.Nets)
	defer rEnv.Teardown()

	rIfc, err := New(nEnv.Nets[0], rEnv.GenRouterConfig(0))
	require.NoError(t, err)

	r, ok := rIfc.(*router)
	require.True(t, ok)

	defer func() {
		require.NoError(t, r.Close())
	}()

	r.rt = rt

	// TEST: Set and get expired and unexpired rule.
	t.Run("GetRule", func(t *testing.T) {
		testGetRule(t, r, rt)
	})

	// TEST: Ensure removing route descriptor works properly.
	t.Run("RemoveRouteDescriptor", func(t *testing.T) {
		testRemoveRouteDescriptor(t, r, rt)
	})
}

func testRemoveRouteDescriptor(t *testing.T, r *router, rt routing.Table) {
	clearRoutingTableRules(rt)

	localPK, _ := cipher.GenerateKeyPair()
	remotePK, _ := cipher.GenerateKeyPair()

	id, err := r.rt.ReserveKeys(1)
	require.NoError(t, err)

	rule := routing.ConsumeRule(10*time.Minute, id[0], localPK, remotePK, 2, 3)
	err = r.rt.SaveRule(rule)
	require.NoError(t, err)

	desc := routing.NewRouteDescriptor(localPK, remotePK, 3, 2)
	r.RemoveRouteDescriptor(desc)
	assert.Equal(t, 1, rt.Count())

	desc = routing.NewRouteDescriptor(localPK, remotePK, 2, 3)
	r.RemoveRouteDescriptor(desc)
	assert.Equal(t, 0, rt.Count())
}

func testGetRule(t *testing.T, r *router, rt routing.Table) {
	clearRoutingTableRules(rt)

	expiredID, err := r.rt.ReserveKeys(1)
	require.NoError(t, err)

	expiredRule := routing.IntermediaryForwardRule(-10*time.Minute, expiredID[0], 3, uuid.New())
	err = r.rt.SaveRule(expiredRule)
	require.NoError(t, err)

	id, err := r.rt.ReserveKeys(1)
	require.NoError(t, err)

	rule := routing.IntermediaryForwardRule(10*time.Minute, id[0], 3, uuid.New())
	err = r.rt.SaveRule(rule)
	require.NoError(t, err)

	defer r.rt.DelRules([]routing.RouteID{id[0], expiredID[0]})

	// rule should already be expired at this point due to the execution time.
	// However, we'll just a bit to be sure
	time.Sleep(1 * time.Millisecond)

	_, err = r.GetRule(expiredID[0])
	require.Error(t, err)

	_, err = r.GetRule(123)
	require.Error(t, err)

	gotRule, err := r.GetRule(id[0])
	require.NoError(t, err)
	assert.Equal(t, rule, gotRule)
}

func TestRouter_SetupIsTrusted(t *testing.T) {
	keys := snettest.GenKeyPairs(2)

	nEnv := snettest.NewEnv(t, keys, []string{dmsg.Type})
	defer nEnv.Teardown()

	rEnv := NewTestEnv(t, nEnv.Nets)
	defer rEnv.Teardown()

	routerConfig := rEnv.GenRouterConfig(0)
	routerConfig.SetupNodes = append(routerConfig.SetupNodes, keys[0].PK)

	r0, err := New(nEnv.Nets[0], routerConfig)
	require.NoError(t, err)

	assert.True(t, r0.SetupIsTrusted(keys[0].PK))
	assert.False(t, r0.SetupIsTrusted(keys[1].PK))
}

func clearRouteGroups(routers ...*router) {
	for _, r := range routers {
		r.rgsNs = make(map[routing.RouteDescriptor]*NoiseRouteGroup)
	}
}

func clearRouterRules(routers ...*router) {
	for _, r := range routers {
		rules := r.rt.AllRules()
		for _, rule := range rules {
			r.rt.DelRules([]routing.RouteID{rule.KeyRouteID()})
		}
	}
}

func clearRoutingTableRules(rt routing.Table) {
	rules := rt.AllRules()
	for _, rule := range rules {
		rt.DelRules([]routing.RouteID{rule.KeyRouteID()})
	}
}

type TestEnv struct {
	TpD transport.DiscoveryClient

	TpMngrConfs []*transport.ManagerConfig
	TpMngrs     []*transport.Manager

	teardown func()
}

func NewTestEnv(t *testing.T, nets []*snet.Network) *TestEnv {
	tpD := transport.NewDiscoveryMock()

	mConfs := make([]*transport.ManagerConfig, len(nets))
	ms := make([]*transport.Manager, len(nets))

	for i, n := range nets {
		var err error

		mConfs[i] = &transport.ManagerConfig{
			PubKey:          n.LocalPK(),
			SecKey:          n.LocalSK(),
			DiscoveryClient: tpD,
			LogStore:        transport.InMemoryTransportLogStore(),
		}

		ms[i], err = transport.NewManager(nil, n, mConfs[i])
		require.NoError(t, err)

		go ms[i].Serve(context.TODO())
	}

	teardown := func() {
		for _, m := range ms {
			assert.NoError(t, m.Close())
		}
	}

	return &TestEnv{
		TpD:         tpD,
		TpMngrConfs: mConfs,
		TpMngrs:     ms,
		teardown:    teardown,
	}
}

func (e *TestEnv) GenRouterConfig(i int) *Config {
	return &Config{
		Logger:           logging.MustGetLogger(fmt.Sprintf("router_%d", i)),
		PubKey:           e.TpMngrConfs[i].PubKey,
		SecKey:           e.TpMngrConfs[i].SecKey,
		TransportManager: e.TpMngrs[i],
		SetupNodes:       nil, // TODO
	}
}

func (e *TestEnv) Teardown() {
	e.teardown()
}
