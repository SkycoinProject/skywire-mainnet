package snettest

import (
	"context"
	"strconv"
	"testing"

	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/disc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"

	"github.com/skycoin/skywire/pkg/snet"
	"github.com/skycoin/skywire/pkg/snet/arclient"
	"github.com/skycoin/skywire/pkg/snet/directtp"
	"github.com/skycoin/skywire/pkg/snet/directtp/pktable"
	"github.com/skycoin/skywire/pkg/snet/directtp/tptypes"
)

// KeyPair holds a public/private key pair.
type KeyPair struct {
	PK cipher.PubKey
	SK cipher.SecKey
}

// GenKeyPairs generates 'n' number of key pairs.
func GenKeyPairs(n int) []KeyPair {
	pairs := make([]KeyPair, n)
	for i := range pairs {
		pk, sk, err := cipher.GenerateDeterministicKeyPair([]byte{byte(i)})
		if err != nil {
			panic(err)
		}

		pairs[i] = KeyPair{PK: pk, SK: sk}
	}

	return pairs
}

// Env contains a network test environment.
type Env struct {
	DmsgD    disc.APIClient
	DmsgS    *dmsg.Server
	Keys     []KeyPair
	Nets     []*snet.Network
	teardown func()
}

// NewEnv creates a `network.Network` test environment.
// `nPairs` is the public/private key pairs of all the `network.Network`s to be created.
func NewEnv(t *testing.T, keys []KeyPair, networks []string) *Env {
	// Prepare `dmsg`.
	dmsgD := disc.NewMock(0)
	dmsgS, dmsgSErr := createDmsgSrv(t, dmsgD)

	const baseSTCPPort = 7033

	tableEntries := make(map[cipher.PubKey]string)
	for i, pair := range keys {
		tableEntries[pair.PK] = "127.0.0.1:" + strconv.Itoa(baseSTCPPort+i)
	}

	table := pktable.NewTable(tableEntries)

	var hasDmsg, hasStcp, hasStcpr, hasSudph bool

	for _, network := range networks {
		switch network {
		case dmsg.Type:
			hasDmsg = true
		case tptypes.STCP:
			hasStcp = true
		case tptypes.STCPR:
			hasStcpr = true
		case tptypes.SUDPH:
			hasSudph = true
		}
	}

	// Prepare `snets`.
	ns := make([]*snet.Network, len(keys))

	const stcpBasePort = 7033

	for i, pairs := range keys {
		networkConfigs := snet.NetworkConfigs{
			Dmsg: &snet.DmsgConfig{
				SessionsCount: 1,
			},
			STCP: &snet.STCPConfig{
				LocalAddr: "127.0.0.1:" + strconv.Itoa(stcpBasePort+i),
			},
		}

		clients := snet.NetworkClients{
			Direct: make(map[string]directtp.Client),
		}

		if hasDmsg {
			clients.DmsgC = dmsg.NewClient(pairs.PK, pairs.SK, dmsgD, nil)
			go clients.DmsgC.Serve(context.Background())
		}

		addressResolver := new(arclient.MockAPIClient)

		if hasStcp {
			conf := directtp.Config{
				Type:      tptypes.STCP,
				PK:        pairs.PK,
				SK:        pairs.SK,
				Table:     table,
				LocalAddr: networkConfigs.STCP.LocalAddr,
			}

			clients.Direct[tptypes.STCP] = directtp.NewClient(conf)
		}

		if hasStcpr {
			conf := directtp.Config{
				Type:            tptypes.STCPR,
				PK:              pairs.PK,
				SK:              pairs.SK,
				AddressResolver: addressResolver,
			}

			clients.Direct[tptypes.STCPR] = directtp.NewClient(conf)
		}

		if hasSudph {
			conf := directtp.Config{
				Type:            tptypes.SUDPH,
				PK:              pairs.PK,
				SK:              pairs.SK,
				AddressResolver: addressResolver,
			}

			clients.Direct[tptypes.SUDPH] = directtp.NewClient(conf)
		}

		snetConfig := snet.Config{
			PubKey:         pairs.PK,
			SecKey:         pairs.SK,
			NetworkConfigs: networkConfigs,
		}

		n := snet.NewRaw(snetConfig, clients)
		require.NoError(t, n.Init(context.Background()))
		ns[i] = n
	}

	// Prepare teardown closure.
	teardown := func() {
		for _, n := range ns {
			assert.NoError(t, n.Close())
		}
		assert.NoError(t, dmsgS.Close())
		for err := range dmsgSErr {
			assert.NoError(t, err)
		}
	}

	return &Env{
		DmsgD:    dmsgD,
		DmsgS:    dmsgS,
		Keys:     keys,
		Nets:     ns,
		teardown: teardown,
	}
}

// Teardown shutdowns the Env.
func (e *Env) Teardown() { e.teardown() }

func createDmsgSrv(t *testing.T, dc disc.APIClient) (srv *dmsg.Server, srvErr <-chan error) {
	pk, sk, err := cipher.GenerateDeterministicKeyPair([]byte("s"))
	require.NoError(t, err)

	l, err := nettest.NewLocalListener("tcp")
	require.NoError(t, err)

	srv = dmsg.NewServer(pk, sk, dc, &dmsg.ServerConfig{MaxSessions: 100}, nil)

	errCh := make(chan error, 1)

	go func() {
		errCh <- srv.Serve(l, "")
		close(errCh)
	}()

	<-srv.Ready()

	return srv, errCh
}
