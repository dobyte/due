package gate

import (
	"context"
	"github.com/dobyte/due/packet"
	"github.com/dobyte/due/session"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/grpc/internal/code"
	"github.com/dobyte/due/transport/grpc/internal/pb"
	"github.com/dobyte/due/transport/grpc/internal/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewServer(provider transport.GateProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	s.RegisterService(&pb.Gate_ServiceDesc, &endpoint{provider: provider})

	return s, nil
}

type endpoint struct {
	pb.UnimplementedGateServer
	provider transport.GateProvider
}

// Bind 将用户与当前网关进行绑定
func (e *endpoint) Bind(ctx context.Context, req *pb.BindRequest) (*pb.BindReply, error) {
	if req.CID <= 0 || req.UID <= 0 {
		return nil, status.New(codes.InvalidArgument, "invalid argument").Err()
	}

	s, err := e.provider.Session(session.Conn, req.CID)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	if err = e.provider.Bind(ctx, req.UID); err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	s.Bind(req.UID)

	return &pb.BindReply{}, nil
}

// Unbind 将用户与当前网关进行解绑
func (e *endpoint) Unbind(ctx context.Context, req *pb.UnbindRequest) (*pb.UnbindReply, error) {
	if req.UID <= 0 {
		return nil, status.New(codes.InvalidArgument, "invalid argument").Err()
	}

	s, err := e.provider.Session(session.User, req.UID)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	if err = e.provider.Unbind(ctx, req.UID); err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	s.Unbind(req.UID)

	return &pb.UnbindReply{}, nil
}

// GetIP 获取客户端IP地址
func (e *endpoint) GetIP(ctx context.Context, req *pb.GetIPRequest) (*pb.GetIPReply, error) {
	s, err := e.provider.Session(session.Kind(req.Kind), req.Target)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
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
	msg, err := packet.Pack(&packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	if err = e.provider.Push(session.Kind(req.Kind), req.Target, msg); err != nil {
		switch err {
		case session.ErrNotFoundSession:
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
	msg, err := packet.Pack(&packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	total, err := e.provider.Multicast(session.Kind(req.Kind), req.Targets, msg)
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
	msg, err := packet.Pack(&packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		return nil, status.New(codes.Internal, err.Error()).Err()
	}

	total, err := e.provider.Broadcast(session.Kind(req.Kind), msg)
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
	s, err := e.provider.Session(session.Kind(req.Kind), req.Target)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
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
