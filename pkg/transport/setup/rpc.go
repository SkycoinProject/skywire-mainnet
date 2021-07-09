package setup

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/pkg/transport"
	"github.com/skycoin/skywire/pkg/transport/network"
)

// TransportGateway that exposes methods to be used via RPC
type TransportGateway struct {
	tm  *transport.Manager
	log *logging.Logger
}

// TransportRequest to perform an action over RPC
type TransportRequest struct {
	RemotePK cipher.PubKey
	Type     network.Type
}

// UUIDRequest contains id in UUID format
type UUIDRequest struct {
	ID uuid.UUID
}

// TransportResponse specifies an existing transport to remote node
type TransportResponse struct {
	ID     uuid.UUID
	Local  cipher.PubKey
	Remote cipher.PubKey
	Type   network.Type
}

// BoolResponse is a simple boolean wrapped in structure for RPC responses
type BoolResponse struct {
	Result bool
}

// AddTransport specified by request
func (gw *TransportGateway) AddTransport(req TransportRequest, res *TransportResponse) error {
	ctx := context.Background()
	gw.log.WithField("PK", req.RemotePK).WithField("type", req.Type).Info("Adding transport")
	tp, err := gw.tm.SaveTransport(ctx, req.RemotePK, req.Type, transport.LabelSkycoin)
	if err != nil {
		gw.log.WithError(err).Error("Cannot save transport")
		return err
	}
	res.ID = tp.Entry.ID
	res.Local = gw.tm.Local()
	res.Remote = tp.Remote()
	res.Type = tp.Type()
	return nil
}

// ErrIncorrectType is returned when transport was not created by transport setup
var ErrIncorrectType = errors.New("transport was not created by skycoin")

// RemoveTransport removes all transports that match given request
func (gw *TransportGateway) RemoveTransport(req UUIDRequest, res *BoolResponse) error {
	gw.log.WithField("ID", req.ID).Info("Removing transport")
	tr, err := gw.tm.GetTransportByID(req.ID)
	if err != nil {
		return err
	}
	if tr.Entry.Label != transport.LabelSkycoin {
		return ErrIncorrectType
	}
	gw.tm.DeleteTransport(req.ID)
	res.Result = true
	return nil
}

// GetTransports returns all transports of this node that have been established by transport setup system
func (gw *TransportGateway) GetTransports(_ struct{}, res *[]TransportResponse) error {
	tps := gw.tm.GetTransportsByLabel(transport.LabelSkycoin)
	for _, tp := range tps {
		tResp := TransportResponse{
			ID:     tp.Entry.ID,
			Local:  gw.tm.Local(),
			Remote: tp.Remote(),
			Type:   tp.Type(),
		}
		*res = append(*res, tResp)
	}
	return nil
}
