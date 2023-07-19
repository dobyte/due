package gate

import (
	"context"
	"github.com/dobyte/due/transport/rpcx/v2/internal/code"
	"github.com/dobyte/due/transport/rpcx/v2/internal/protocol"
	"github.com/dobyte/due/transport/rpcx/v2/internal/server"
	"github.com/dobyte/due/v2/cluster/gate"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
)

const (
	ServicePath             = "Gate"
	serviceMethodBind       = "Bind"
	serviceMethodUnbind     = "Unbind"
	serviceMethodGetIP      = "GetIP"
	serviceMethodPush       = "Push"
	serviceMethodMulticast  = "Multicast"
	serviceMethodBroadcast  = "Broadcast"
	serviceMethodStat       = "Stat"
	serviceMethodDisconnect = "Disconnect"
)

func NewServer(provider transport.GateProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	err = s.RegisterService(ServicePath, &endpoint{provider: provider})
	if err != nil {
		return nil, err
	}

	return s, nil
}

type endpoint struct {
	provider transport.GateProvider
}

// Bind 将用户与当前网关进行绑定
func (e *endpoint) Bind(ctx context.Context, req *protocol.BindRequest, reply *protocol.BindReply) error {
	err := e.provider.Bind(ctx, req.CID, req.UID)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			reply.Code = code.NotFoundSession
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		case gate.ErrInvalidArgument:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
	}

	return err
}

// Unbind 将用户与当前网关进行解绑
func (e *endpoint) Unbind(ctx context.Context, req *protocol.UnbindRequest, reply *protocol.UnbindReply) error {
	err := e.provider.Unbind(ctx, req.UID)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			reply.Code = code.NotFoundSession
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		case gate.ErrInvalidArgument:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
	}

	return err
}

// GetIP 获取客户端IP地址
func (e *endpoint) GetIP(ctx context.Context, req *protocol.GetIPRequest, reply *protocol.GetIPReply) error {
	ip, err := e.provider.GetIP(ctx, req.Kind, req.Target)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			reply.Code = code.NotFoundSession
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		case gate.ErrInvalidArgument:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
	}

	reply.IP = ip

	return err
}

// Push 推送消息给连接
func (e *endpoint) Push(ctx context.Context, req *protocol.PushRequest, reply *protocol.PushReply) error {
	err := e.provider.Push(ctx, req.Kind, req.Target, req.Message)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			reply.Code = code.NotFoundSession
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
	}

	return err
}

// Multicast 推送组播消息
func (e *endpoint) Multicast(ctx context.Context, req *protocol.MulticastRequest, reply *protocol.MulticastReply) error {
	total, err := e.provider.Multicast(ctx, req.Kind, req.Targets, req.Message)
	if err != nil {
		switch err {
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
	}

	reply.Total = total

	return err
}

// Broadcast 推送广播消息
func (e *endpoint) Broadcast(ctx context.Context, req *protocol.BroadcastRequest, reply *protocol.BroadcastReply) error {
	total, err := e.provider.Broadcast(ctx, req.Kind, req.Message)
	if err != nil {
		switch err {
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
	}

	reply.Total = total

	return err
}

// Stat 统计会话总数
func (e *endpoint) Stat(ctx context.Context, req *protocol.StatRequest, reply *protocol.StatReply) error {
	total, err := e.provider.Stat(ctx, req.Kind)
	if err != nil {
		switch err {
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
	}

	reply.Total = total

	return err
}

// Disconnect 断开连接
func (e *endpoint) Disconnect(ctx context.Context, req *protocol.DisconnectRequest, reply *protocol.DisconnectReply) error {
	err := e.provider.Disconnect(ctx, req.Kind, req.Target, req.IsForce)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			reply.Code = code.NotFoundSession
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
	}

	return err
}
