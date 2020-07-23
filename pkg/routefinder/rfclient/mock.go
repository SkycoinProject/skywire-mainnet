package rfclient

import (
	"fmt"
	"net/http"

	"github.com/SkycoinProject/dmsg/cipher"
	"golang.org/x/net/context"

	"github.com/SkycoinProject/skywire-mainnet/pkg/routing"
	"github.com/SkycoinProject/skywire-mainnet/pkg/transport"
)

// MockClient implements mock route finder client.
type mockClient struct {
	err error
}

// NewMock constructs a new mock Client.
func NewMock() Client {
	return &mockClient{}
}

// SetError assigns error that will be return on the next call to a
// public method.
func (r *mockClient) SetError(err error) {
	r.err = err
}

// FindRoutes implements Client for MockClient
func (r *mockClient) FindRoutes(ctx context.Context, rts []routing.PathEdges, opts *RouteOptions) (map[routing.PathEdges][][]routing.Hop, error) {
	if r.err != nil {
		return nil, r.err
	}

	if len(rts) == 0 {
		return nil, fmt.Errorf("no edges provided to returns routes from")
	}

	return map[routing.PathEdges][][]routing.Hop{
		[2]cipher.PubKey{rts[0][0], rts[0][1]}: {
			{
				routing.Hop{
					TpID: transport.MakeTransportID(rts[0][0], rts[0][1], ""),
					From: rts[0][0],
					To:   rts[0][1],
				},
			},
		},
	}, nil
}

// Health implements Client for MockClient
func (r *mockClient) Health(_ context.Context) (int, error) {
	return http.StatusOK, nil
}