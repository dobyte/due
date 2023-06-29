package node

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/grpc/internal/code"
	"github.com/dobyte/due/transport/grpc/internal/pb"
	"github.com/dobyte/due/transport/grpc/internal/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewServer(provider transport.NodeProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	err = s.RegisterService(&pb.Node_ServiceDesc, &endpoint{provider: provider})
	if err != nil {
		return nil, err
	}

	return s, nil
}

type endpoint struct {
	pb.UnimplementedNodeServer
	provider transport.NodeProvider
}

// Trigger 触发事件
func (e *endpoint) Trigger(ctx context.Context, req *pb.TriggerRequest) (*pb.TriggerReply, error) {
	miss, err := e.provider.Trigger(ctx, &transport.TriggerArgs{
		GID:   req.GID,
		CID:   req.CID,
		UID:   req.UID,
		Event: cluster.Event(req.Event),
	})
	if err != nil {
		if miss {
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		} else {
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.TriggerReply{}, nil
}

// Deliver 投递消息
func (e *endpoint) Deliver(ctx context.Context, req *pb.DeliverRequest) (*pb.DeliverReply, error) {
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
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		} else {
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.DeliverReply{}, nil
}
