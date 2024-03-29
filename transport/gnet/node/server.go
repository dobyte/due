package node

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/transport/gnet/internal/pb"
	"github.com/symsimmy/due/transport/gnet/internal/server"
)

type endpoint struct {
	provider transport.NodeProvider
}

func NewServer(provider transport.NodeProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}
	endpoint := endpoint{provider: provider}
	s.OnReceive(endpoint.dispatch)
	return s, nil
}

func (e *endpoint) dispatch(methodName uint16, data []byte) (reply interface{}, err error) {
	switch methodName {
	case transport.Trigger:
		req := &pb.TriggerRequest{}
		err := proto.Unmarshal(data, req)
		if err != nil {
			return nil, err
		}
		_, err = e.provider.Trigger(context.Background(), &transport.TriggerArgs{
			GID:   req.GID,
			CID:   req.CID,
			UID:   req.UID,
			Event: cluster.Event(req.Event),
		})
		if err != nil {
			return nil, err
		}
		break
	case transport.Deliver:
		req := &pb.DeliverRequest{}
		err := proto.Unmarshal(data, req)
		if err != nil {
			return nil, err
		}
		_, err = e.provider.Deliver(context.Background(), &transport.DeliverArgs{
			GID: req.GID,
			NID: req.NID,
			CID: req.CID,
			UID: req.UID,
			Message: &transport.Message{
				Seq:    req.Message.Seq,
				Route:  req.Message.Route,
				Buffer: req.Message.Buffer,
			},
		})

		break
	}
	return
}
