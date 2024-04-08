package node

import (
	"context"
	"github.com/dobyte/due/transport/kitex/v2/internal/protocol/node"
	inner "github.com/dobyte/due/transport/kitex/v2/internal/protocol/node/node"
	"github.com/dobyte/due/transport/kitex/v2/internal/server"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/transport"
)

func NewServer(provider transport.NodeProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	err = s.RegisterService(inner.NewServiceInfo(), &endpoint{provider: provider})
	if err != nil {
		return nil, err
	}

	return s, nil
}

type endpoint struct {
	provider transport.NodeProvider
}

// Trigger 触发事件
func (e *endpoint) Trigger(ctx context.Context, req *node.TriggerRequest) (*node.TriggerResponse, error) {
	miss, err := e.provider.Trigger(ctx, &transport.TriggerArgs{
		GID:   req.GID,
		CID:   req.CID,
		UID:   req.UID,
		Event: int(req.Event),
	})
	if err != nil {
		if miss {
			return nil, errors.New("miss")
		} else {
			return nil, errors.New("internal")
		}
	}

	return &node.TriggerResponse{}, nil
}

// Deliver 投递消息
func (e *endpoint) Deliver(ctx context.Context, req *node.DeliverRequest) (*node.DeliverResponse, error) {
	miss, err := e.provider.Deliver(ctx, &transport.DeliverArgs{
		GID: req.GID,
		NID: req.NID,
		CID: req.CID,
		UID: req.UID,
		Message: &packet.Message{
			Seq:    req.Message.Seq,
			Route:  req.Message.Route,
			Buffer: req.Message.Buffer,
		},
	})
	if err != nil {
		if miss {
			return nil, errors.New("miss")
		} else {
			return nil, errors.New("internal")
		}
	}

	return &node.DeliverResponse{}, nil
}
