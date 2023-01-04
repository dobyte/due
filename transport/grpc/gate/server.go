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
	err := e.provider.Bind(ctx, req.CID, req.CID)
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

	return &pb.BindReply{}, nil
}

// Unbind 将用户与当前网关进行解绑
func (e *endpoint) Unbind(ctx context.Context, req *pb.UnbindRequest) (*pb.UnbindReply, error) {
	err := e.provider.Unbind(ctx, req.UID)
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

	return &pb.UnbindReply{}, nil
}

// GetIP 获取客户端IP地址
func (e *endpoint) GetIP(_ context.Context, req *pb.GetIPRequest) (*pb.GetIPReply, error) {
	ip, err := e.provider.GetIP(session.Kind(req.Kind), req.Target)
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

	return &pb.GetIPReply{IP: ip}, nil
}

// Push 推送消息给连接
func (e *endpoint) Push(_ context.Context, req *pb.PushRequest) (*pb.PushReply, error) {
	err := e.provider.Push(session.Kind(req.Kind), req.Target, &packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
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

	return &pb.PushReply{}, nil
}

// Multicast 推送组播消息
func (e *endpoint) Multicast(_ context.Context, req *pb.MulticastRequest) (*pb.MulticastReply, error) {
	total, err := e.provider.Multicast(session.Kind(req.Kind), req.Targets, &packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		switch err {
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.MulticastReply{Total: total}, nil
}

// Broadcast 推送广播消息
func (e *endpoint) Broadcast(_ context.Context, req *pb.BroadcastRequest) (*pb.BroadcastReply, error) {
	total, err := e.provider.Broadcast(session.Kind(req.Kind), &packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		switch err {
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.BroadcastReply{Total: total}, nil
}

// Disconnect 断开连接
func (e *endpoint) Disconnect(_ context.Context, req *pb.DisconnectRequest) (*pb.DisconnectReply, error) {
	err := e.provider.Disconnect(session.Kind(req.Kind), req.Target, req.IsForce)
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

	return &pb.DisconnectReply{}, nil
}
