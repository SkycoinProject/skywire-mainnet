package transport_test

import (
	"context"
	"testing"

	"github.com/SkycoinProject/dmsg"
	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/skywire-mainnet/internal/skyenv"
	"github.com/SkycoinProject/skywire-mainnet/pkg/snet/snettest"
	"github.com/SkycoinProject/skywire-mainnet/pkg/transport"
)

func TestSettlementHS(t *testing.T) {
	tpDisc := transport.NewDiscoveryMock()

	keys := snettest.GenKeyPairs(2)
	nEnv := snettest.NewEnv(t, keys, []string{dmsg.Type})
	defer nEnv.Teardown()

	// TEST: Perform a handshake between two snet.Network instances.
	t.Run("Do", func(t *testing.T) {
		lis1, err := nEnv.Nets[1].Listen(dmsg.Type, skyenv.DmsgTransportPort)
		require.NoError(t, err)

		errCh1 := make(chan error, 1)
		go func() {
			defer close(errCh1)
			conn1, err := lis1.AcceptConn()
			if err != nil {
				errCh1 <- err
				return
			}
			errCh1 <- transport.MakeSettlementHS(false).Do(context.TODO(), tpDisc, conn1, keys[1].SK)
		}()
		defer func() {
			require.NoError(t, <-errCh1)
		}()

		conn0, err := nEnv.Nets[0].Dial(context.TODO(), dmsg.Type, keys[1].PK, skyenv.DmsgTransportPort)
		require.NoError(t, err)
		require.NoError(t, transport.MakeSettlementHS(true).Do(context.TODO(), tpDisc, conn0, keys[0].SK), "fucked up")
	})
}

// TODO(evanlinjin): This will need further testing.
