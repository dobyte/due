package node

import (
	"context"
	"errors"
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
	if req.UID <= 0 {
		reply.Code = code.InvalidArgument
		return errors.New("invalid argument")
	}

	_, miss, err := e.provider.LocateNode(ctx, req.UID)
	if err != nil {
		if miss {
			reply.Code = code.NotFoundSession
		} else {
			reply.Code = code.Internal
		}
		return err
	}

	e.provider.Trigger(req.Event, req.GID, req.UID)

	return nil
}

// Deliver 投递消息
func (e *endpoint) Deliver(ctx context.Context, req *protocol.DeliverRequest, reply *protocol.DeliverReply) error {
	stateful, ok := e.provider.CheckRouteStateful(req.Message.Route)
	if !ok {
		return nil
	}

	if stateful {
		if req.UID <= 0 {
			reply.Code = code.InvalidArgument
			return errors.New("invalid argument")
		}

		_, miss, err := e.provider.LocateNode(ctx, req.UID)
		if err != nil {
			if miss {
				reply.Code = code.NotFoundSession
			} else {
				reply.Code = code.Internal
			}
			return err
		}
	}

	e.provider.Deliver(req.GID, req.NID, req.CID, req.UID, &transport.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})

	return nil
}
