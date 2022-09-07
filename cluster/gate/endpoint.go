package gate

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dobyte/due/cluster/internal/code"
	"github.com/dobyte/due/cluster/internal/pb"
	"github.com/dobyte/due/packet"
	"github.com/dobyte/due/session"
)

type endpoint struct {
	pb.UnimplementedGateServer
	gate *Gate
}

// Bind 将用户与当前网关进行绑定
func (e *endpoint) Bind(ctx context.Context, req *pb.BindRequest) (*pb.BindReply, error) {
	if req.CID <= 0 || req.UID <= 0 {
		return nil, status.New(codes.InvalidArgument, "invalid argument").Err()
	}

	s, err := e.gate.group.GetSession(session.Conn, req.CID)
	if err != nil {
		switch err {
		case session.ErrSessionNotFound:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	if err = e.gate.proxy.bindGate(ctx, req.UID); err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	s.Bind(req.UID)

	return &pb.BindReply{}, nil
}

// GetIP 获取客户端IP地址
func (e *endpoint) GetIP(ctx context.Context, req *pb.GetIPRequest) (*pb.GetIPReply, error) {
	s, err := e.gate.group.GetSession(session.Kind(req.Kind), req.Target)
	if err != nil {
		switch err {
		case session.ErrSessionNotFound:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	ip, err := s.RemoteIP()
	if err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	return &pb.GetIPReply{IP: ip}, nil
}

// Push 推送消息给连接
func (e *endpoint) Push(ctx context.Context, req *pb.PushRequest) (*pb.PushReply, error) {
	msg, err := packet.Pack(&packet.Message{Route: req.Route, Buffer: req.Buffer})
	if err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	if err = e.gate.group.Push(session.Kind(req.Kind), req.Target, msg); err != nil {
		switch err {
		case session.ErrSessionNotFound:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.PushReply{}, nil
}

// Multicast 推送组播消息
func (e *endpoint) Multicast(ctx context.Context, req *pb.MulticastRequest) (*pb.MulticastReply, error) {
	msg, err := packet.Pack(&packet.Message{Route: req.Route, Buffer: req.Buffer})
	if err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	total, err := e.gate.group.Multicast(session.Kind(req.Kind), req.Targets, msg)
	if err != nil {
		switch err {
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.MulticastReply{Total: int64(total)}, nil
}

// Broadcast 推送广播消息
func (e *endpoint) Broadcast(ctx context.Context, req *pb.BroadcastRequest) (*pb.BroadcastReply, error) {
	msg, err := packet.Pack(&packet.Message{Route: req.Route, Buffer: req.Buffer})
	if err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	total, err := e.gate.group.Broadcast(session.Kind(req.Kind), msg)
	if err != nil {
		switch err {
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.BroadcastReply{Total: int64(total)}, nil
}

// Disconnect 断开连接
func (e *endpoint) Disconnect(ctx context.Context, req *pb.DisconnectRequest) (*pb.DisconnectReply, error) {
	s, err := e.gate.group.GetSession(session.Kind(req.Kind), req.Target)
	if err != nil {
		switch err {
		case session.ErrSessionNotFound:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	if err = s.Close(req.IsForce); err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	return &pb.DisconnectReply{}, nil
}
