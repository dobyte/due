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

	s.RegisterService(&pb.Node_ServiceDesc, &endpoint{provider: provider})

	return s, nil
}

type endpoint struct {
	pb.UnimplementedNodeServer
	provider transport.NodeProvider
}

// Trigger 触发事件
func (e *endpoint) Trigger(ctx context.Context, req *pb.TriggerRequest) (*pb.TriggerReply, error) {
	if req.UID <= 0 {
		return nil, status.New(codes.InvalidArgument, "invalid argument").Err()
	}

	_, miss, err := e.provider.LocateNode(ctx, req.UID)
	if err != nil {
		if miss {
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		} else {
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	e.provider.Trigger(cluster.Event(req.Event), req.GID, req.UID)

	return &pb.TriggerReply{}, nil
}

// Deliver 投递消息
func (e *endpoint) Deliver(ctx context.Context, req *pb.DeliverRequest) (*pb.DeliverReply, error) {
	stateful, ok := e.provider.CheckRouteStateful(req.Message.Route)
	if !ok {
		return &pb.DeliverReply{}, nil
	}

	if stateful {
		if req.UID <= 0 {
			return nil, status.New(codes.InvalidArgument, "invalid argument").Err()
		}

		_, miss, err := e.provider.LocateNode(ctx, req.UID)
		if err != nil {
			if miss {
				return nil, status.New(code.NotFoundSession, err.Error()).Err()
			} else {
				return nil, status.New(codes.Internal, err.Error()).Err()
			}
		}
	}

	e.provider.Deliver(req.GID, req.NID, req.CID, req.UID, &transport.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})

	return &pb.DeliverReply{}, nil
}
