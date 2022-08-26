/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/6/18 12:16 下午
 * @Desc: TODO
 */

package node

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/cluster/internal/code"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dobyte/due/cluster/internal/pb"
)

type endpoint struct {
	pb.UnimplementedNodeServer
	node *Node
}

// Trigger 触发事件
func (e *endpoint) Trigger(ctx context.Context, req *pb.TriggerRequest) (*pb.TriggerReply, error) {
	if req.UID <= 0 {
		return nil, status.New(codes.InvalidArgument, "invalid argument").Err()
	}

	nid, err := e.node.proxy.LocateNode(ctx, req.UID)
	if err != nil {
		switch err {
		case ErrNotFoundUserSource:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	if e.node.opts.id != nid {
		return nil, status.New(code.NotFoundSession, ErrNotFoundUserSource.Error()).Err()
	}

	e.node.trigger(cluster.Event(req.Event), req.GID, req.UID)

	return &pb.TriggerReply{}, nil
}

// Deliver 投递消息
func (e *endpoint) Deliver(ctx context.Context, req *pb.DeliverRequest) (*pb.DeliverReply, error) {
	stateful, ok := e.node.isStatefulRoute(req.Route)
	if !ok {
		return &pb.DeliverReply{}, nil
	}

	if stateful {
		if req.UID <= 0 {
			return nil, status.New(codes.InvalidArgument, "invalid argument").Err()
		}

		nid, err := e.node.proxy.LocateNode(ctx, req.UID)
		if err != nil {
			switch err {
			case ErrNotFoundUserSource:
				return nil, status.New(code.NotFoundSession, err.Error()).Err()
			default:
				return nil, status.New(codes.Internal, err.Error()).Err()
			}
		}

		if e.node.opts.id != nid {
			return nil, status.New(code.NotFoundSession, ErrNotFoundUserSource.Error()).Err()
		}
	}

	e.node.deliver(req.GID, req.NID, req.CID, req.UID, req.Route, req.Buffer)

	return &pb.DeliverReply{}, nil
}
