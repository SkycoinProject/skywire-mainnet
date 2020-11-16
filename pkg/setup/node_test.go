package setup

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skycoin/skywire/pkg/routing"
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
		logging.Disable()
	}

	os.Exit(m.Run())
}

func TestCreateRouteGroup(t *testing.T) {
	pkA, _ := cipher.GenerateKeyPair()
	pkB, _ := cipher.GenerateKeyPair()
	pkC, _ := cipher.GenerateKeyPair()
	pkD, _ := cipher.GenerateKeyPair()

	type testCase struct {
		fwdPKs  []cipher.PubKey
		revPKs  []cipher.PubKey
		SrcPort routing.Port
		DstPort routing.Port
	}

	testCases := []testCase{
		{
			fwdPKs:  []cipher.PubKey{pkA, pkB, pkC, pkD},
			revPKs:  []cipher.PubKey{pkD, pkC, pkB, pkA},
			SrcPort: 1,
			DstPort: 5,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// arrange: router keys
			routerPKs := append(tc.fwdPKs, tc.revPKs...)
			routerCount := countUniquePKs(append(tc.fwdPKs, tc.revPKs...))
			initPK := routerPKs[0]

			// arrange: routers
			routers := make(map[cipher.PubKey]interface{}, routerCount)
			for _, pk := range routerPKs {
				routers[pk] = newMockRouterGateway(pk)
			}

			// arrange: mock dialer
			dialer := newMockDialer(t, routers)

			// arrange: bidirectional route input
			biRt := biRouteFromKeys(tc.fwdPKs, tc.revPKs, tc.SrcPort, tc.DstPort)

			// act
			resp, err := CreateRouteGroupMultiple(context.TODO(), dialer, biRt)
			if err == nil {
				// if successful, inject response (response edge rules) to responding router
				var ok bool
				_ = routers[initPK].(*mockRouterGateway).AddEdgeRules(resp, &ok) // nolint:errcheck
			}

			// assert: no error
			assert.NoError(t, err)

			// assert: valid route ID keys
			for pk, r := range routers {
				mr := r.(*mockRouterGateway)
				t.Logf("Checking router %s: lastRtID=%d edgeRules=%d interRules=%d",
					pk, mr.lastRtID, len(mr.edgeRules), len(mr.interRules))
				checkRtIDKeysOfRouterRules(t, mr)
			}

			// TODO: assert: edge routers
			// * Ensure edge routers have 1 edge rule each, and no inter rules.
			// * Edge rule's descriptor should be of provided src/dst pk/port.

			// TODO: assert: inter routers
			// * Ensure inter routers have 2 or more inter rules (depending on routes).
			// * Ensure inter routers have no edge rules.
		})
	}
}

// checkRtIDKeysOfRouterRules ensures that the rules advertised to the router (from the setup logic) has route ID keys
// which are valid.
func checkRtIDKeysOfRouterRules(t *testing.T, r *mockRouterGateway) {
	r.mx.Lock()
	defer r.mx.Unlock()

	var rtIDKeys []routing.RouteID

	for _, edge := range r.edgeRules {
		rtIDKeys = append(rtIDKeys, edge.Forward[0].KeyRouteID(), edge.Reverse[0].KeyRouteID())
	}
	for _, rules := range r.interRules {
		for _, rule := range rules {
			rtIDKeys = append(rtIDKeys, rule.KeyRouteID())
		}
	}

	// assert: no duplicate rtIDs
	dupM := make(map[routing.RouteID]struct{})
	for _, rtID := range rtIDKeys {
		dupM[rtID] = struct{}{}
	}
	assert.Len(t, dupM, len(rtIDKeys), "rtIDKeys=%v dupM=%v", rtIDKeys, dupM)

	// assert: all routes IDs are explicitly reserved by router
	for _, rtID := range rtIDKeys {
		assert.LessOrEqual(t, uint32(rtID), r.lastRtID)
	}
}

func countUniquePKs(pks []cipher.PubKey) int {
	m := make(map[cipher.PubKey]struct{})
	for _, pk := range pks {
		m[pk] = struct{}{}
	}
	return len(m)
}

func biRouteFromKeys(fwdPKs, revPKs []cipher.PubKey, srcPort, dstPort routing.Port) routing.BidirectionalRouteList {
	fwdHops := make([]routing.Hop, len(fwdPKs)-1)
	for i, srcPK := range fwdPKs[:len(fwdPKs)-1] {
		dstPK := fwdPKs[i+1]
		fwdHops[i] = routing.Hop{TpID: determineTpID(srcPK, dstPK), From: srcPK, To: dstPK}
	}

	revHops := make([]routing.Hop, len(revPKs)-1)
	for i, srcPK := range revPKs[:len(revPKs)-1] {
		dstPK := revPKs[i+1]
		revHops[i] = routing.Hop{TpID: determineTpID(srcPK, dstPK), From: srcPK, To: dstPK}
	}

	// TODO: This should also return a map of format: map[uuid.UUID][]cipher.PubKey
	// This way, we can associate transport IDs to the two transport edges, allowing for more checks.
	return routing.BidirectionalRouteList{
		Desc:      routing.NewRouteDescriptor(fwdPKs[0], revPKs[0], srcPort, dstPort),
		KeepAlive: 0,
		Forward:   [][]routing.Hop{fwdHops},
		Reverse:   [][]routing.Hop{revHops},
	}
}

// for tests, we make transport IDs deterministic
// hence, we can derive the tpID from any pk pair
func determineTpID(pk1, pk2 cipher.PubKey) (tpID uuid.UUID) {
	v1, v2 := pk1.Big(), pk2.Big()

	var hash cipher.SHA256
	if v1.Cmp(v2) > 0 {
		hash = cipher.SumSHA256(append(pk1[:], pk2[:]...))
	} else {
		hash = cipher.SumSHA256(append(pk2[:], pk1[:]...))
	}

	copy(tpID[:], hash[:])
	return tpID
}

// mockRouterGateway mocks router.RPCGateway and has an internal state machine that records all remote calls.
// mockRouterGateway acts as a well behaved router, and no error will be returned on any of it's endpoints.
type mockRouterGateway struct {
	pk         cipher.PubKey           // router's public key
	lastRtID   uint32                  // last route ID that was reserved (the first returned rtID would be 1 if this starts as 0).
	edgeRules  []routing.EdgeRulesList // edge rules added by remote.
	interRules [][]routing.Rule        // intermediary rules added by remote.
	mx         sync.Mutex
}

func newMockRouterGateway(pk cipher.PubKey) *mockRouterGateway {
	return &mockRouterGateway{pk: pk}
}

func (gw *mockRouterGateway) AddEdgeRules(rules routing.EdgeRulesList, ok *bool) error {
	gw.mx.Lock()
	defer gw.mx.Unlock()

	gw.edgeRules = append(gw.edgeRules, rules)
	*ok = true
	return nil
}

func (gw *mockRouterGateway) AddIntermediaryRules(rules []routing.Rule, ok *bool) error {
	gw.mx.Lock()
	defer gw.mx.Unlock()

	gw.interRules = append(gw.interRules, rules)
	*ok = true
	return nil
}

func (gw *mockRouterGateway) ReserveIDs(n uint8, routeIDs *[]routing.RouteID) error {
	gw.mx.Lock()
	defer gw.mx.Unlock()

	out := make([]routing.RouteID, n)
	for i := range out {
		gw.lastRtID++
		out[i] = routing.RouteID(gw.lastRtID)
	}
	*routeIDs = out
	return nil
}

// There are no distinctive goals for this test yet.
// As of writing, we only check whether GenerateRules() returns any errors.
func TestGenerateRules(t *testing.T) {
	pkA, _ := cipher.GenerateKeyPair()
	pkB, _ := cipher.GenerateKeyPair()
	pkC, _ := cipher.GenerateKeyPair()
	pkD, _ := cipher.GenerateKeyPair()

	type testCase struct {
		fwd routing.Route
		rev routing.Route
	}

	testCases := []testCase{
		{
			fwd: routing.Route{
				Desc: routing.NewRouteDescriptor(pkA, pkC, 1, 0),
				Hops: []routing.Hop{
					{TpID: uuid.New(), From: pkA, To: pkB},
					{TpID: uuid.New(), From: pkB, To: pkD},
					{TpID: uuid.New(), From: pkD, To: pkC},
				},
			},
			rev: routing.Route{
				Desc: routing.NewRouteDescriptor(pkC, pkA, 0, 1),
				Hops: []routing.Hop{
					{TpID: uuid.New(), From: pkC, To: pkB},
					{TpID: uuid.New(), From: pkB, To: pkA},
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// arrange
			rtIDR := newMockReserver(t, nil)

			// act
			fwd, rev, inter, err := GenerateRules(rtIDR, []routing.Route{tc.fwd, tc.rev})
			t.Log("FORWARD:", fwd)
			t.Log("REVERSE:", rev)
			t.Log("INTERMEDIARY:", inter)

			// assert
			// TODO: We need more checks here
			require.NoError(t, err)
			require.Len(t, fwd, 2)
			require.Len(t, rev, 2)
		})
	}
}

func TestBroadcastIntermediaryRules(t *testing.T) {
	const ctxTimeout = time.Second
	const failingTimeout = time.Second * 5

	type testCase struct {
		workingRouters int // number of working routers
		failingRouters int // number of failing routers
	}

	testCases := []testCase{
		{workingRouters: 4, failingRouters: 0},
		{workingRouters: 12, failingRouters: 1},
		{workingRouters: 9, failingRouters: 2},
		{workingRouters: 0, failingRouters: 3},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%d_normal_%d_failing", tc.workingRouters, tc.failingRouters)

		t.Run(name, func(t *testing.T) {
			// arrange
			workingPKs := randPKs(tc.workingRouters)
			failingPKs := randPKs(tc.failingRouters)

			gateways := make(map[cipher.PubKey]interface{}, tc.workingRouters+tc.failingRouters)
			for _, pk := range workingPKs {
				gateways[pk] = &mockGatewayForReserver{}
			}
			for _, pk := range failingPKs {
				gateways[pk] = &mockGatewayForReserver{hangDuration: failingTimeout}
			}

			rtIDR := newMockReserver(t, gateways)
			rules := randRulesMap(append(workingPKs, failingPKs...))

			ctx, cancel := context.WithDeadline(context.TODO(), time.Now().Add(ctxTimeout))
			defer cancel()

			// act
			err := BroadcastIntermediaryRules(ctx, logrus.New(), rtIDR, rules)

			// assert
			if tc.failingRouters > 0 {
				assert.EqualError(t, err, context.DeadlineExceeded.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func randPKs(n int) []cipher.PubKey {
	out := make([]cipher.PubKey, n)
	for i := range out {
		out[i], _ = cipher.GenerateKeyPair()
	}
	return out
}

func randRulesMap(pks []cipher.PubKey) RulesMap {
	rules := make(RulesMap, len(pks))
	for _, pk := range pks {
		rules[pk] = randIntermediaryRules(2)
	}
	return rules
}

func randIntermediaryRules(n int) []routing.Rule {
	const keepAlive = time.Second
	randRtID := func() routing.RouteID { return routing.RouteID(rand.Uint32()) } // nolint: gosec

	out := make([]routing.Rule, n)
	for i := range out {
		out[i] = routing.IntermediaryForwardRule(keepAlive, randRtID(), randRtID(), uuid.New())
	}
	return out
}
