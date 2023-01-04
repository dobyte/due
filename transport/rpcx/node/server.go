package node

import (
	"context"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/rpcx/internal/code"
	"github.com/dobyte/due/transport/rpcx/internal/protocol"
	"github.com/dobyte/due/transport/rpcx/internal/server"
)

const (
	servicePath          = "Node"
	serviceTriggerMethod = "Trigger"
	serviceDeliverMethod = "Deliver"
)

func NewServer(provider transport.NodeProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	s.RegisterService(servicePath, &endpoint{provider: provider})

	return s, nil
}

type endpoint struct {
	provider transport.NodeProvider
}

// Trigger 触发事件
func (e *endpoint) Trigger(ctx context.Context, req *protocol.TriggerRequest, reply *protocol.TriggerReply) error {
	miss, err := e.provider.Trigger(ctx, &transport.TriggerArgs{
		Event: req.Event,
		GID:   req.GID,
		UID:   req.UID,
	})
	if err != nil {
		if miss {
			reply.Code = code.NotFoundSession
		} else {
			reply.Code = code.Internal
		}
	}

	return err
}

// Deliver 投递消息
func (e *endpoint) Deliver(ctx context.Context, req *protocol.DeliverRequest, reply *protocol.DeliverReply) error {
	miss, err := e.provider.Deliver(ctx, &transport.DeliverArgs{
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
	if err != nil {
		if miss {
			reply.Code = code.NotFoundSession
		} else {
			reply.Code = code.Internal
		}
	}

	return err
}
