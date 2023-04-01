package gate

import (
	"context"
	"github.com/dobyte/due/cluster/gate"
	"github.com/dobyte/due/packet"
	"github.com/dobyte/due/session"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/rpcx/internal/code"
	"github.com/dobyte/due/transport/rpcx/internal/protocol"
	"github.com/dobyte/due/transport/rpcx/internal/server"
	"github.com/dobyte/due/transport/rpcx/node"
)

const (
	ServicePath             = "Gate"
	serviceMethodBind       = "Bind"
	serviceMethodUnbind     = "Unbind"
	serviceMethodGetIP      = "GetIP"
	serviceMethodPush       = "Push"
	serviceMethodMulticast  = "Multicast"
	serviceMethodBroadcast  = "Broadcast"
	serviceMethodDisconnect = "Disconnect"
)

func NewServer(provider transport.GateProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	err = s.RegisterSystemService(ServicePath, &endpoint{provider: provider}, []string{ServicePath, node.ServicePath})
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
func (e *endpoint) GetIP(_ context.Context, req *protocol.GetIPRequest, reply *protocol.GetIPReply) error {
	ip, err := e.provider.GetIP(req.Kind, req.Target)
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
func (e *endpoint) Push(_ context.Context, req *protocol.PushRequest, reply *protocol.PushReply) error {
	err := e.provider.Push(req.Kind, req.Target, &packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
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
func (e *endpoint) Multicast(_ context.Context, req *protocol.MulticastRequest, reply *protocol.MulticastReply) error {
	total, err := e.provider.Multicast(req.Kind, req.Targets, &packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
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
func (e *endpoint) Broadcast(_ context.Context, req *protocol.BroadcastRequest, reply *protocol.BroadcastReply) error {
	total, err := e.provider.Broadcast(req.Kind, &packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
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
func (e *endpoint) Disconnect(_ context.Context, req *protocol.DisconnectRequest, reply *protocol.DisconnectReply) error {
	err := e.provider.Disconnect(req.Kind, req.Target, req.IsForce)
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
