package gate

import (
	"context"
	"github.com/dobyte/due/errors"
	"github.com/dobyte/due/packet"
	"github.com/dobyte/due/session"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/rpcx/internal/code"
	"github.com/dobyte/due/transport/rpcx/internal/protocol"
	"github.com/dobyte/due/transport/rpcx/internal/server"
)

const (
	servicePath             = "Gate"
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

	s.RegisterService(servicePath, &endpoint{provider: provider})

	return s, nil
}

type endpoint struct {
	provider transport.GateProvider
}

// Bind 将用户与当前网关进行绑定
func (e *endpoint) Bind(ctx context.Context, req *protocol.BindRequest, reply *protocol.BindReply) error {
	if req.CID <= 0 || req.UID <= 0 {
		reply.Code = code.InvalidArgument
		return errors.New("invalid argument")
	}

	s, err := e.provider.Session(session.Conn, req.CID)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			reply.Code = code.NotFoundSession
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
		return err
	}

	if err = e.provider.Bind(ctx, req.UID); err != nil {
		reply.Code = code.Internal
		return err
	}

	s.Bind(req.UID)

	return nil
}

// Unbind 将用户与当前网关进行解绑
func (e *endpoint) Unbind(ctx context.Context, req *protocol.UnbindRequest, reply *protocol.UnbindReply) error {
	if req.UID <= 0 {
		reply.Code = code.InvalidArgument
		return errors.New("invalid argument")
	}

	s, err := e.provider.Session(session.User, req.UID)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			reply.Code = code.NotFoundSession
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
		return err
	}

	if err = e.provider.Unbind(ctx, req.UID); err != nil {
		reply.Code = code.Internal
		return err
	}

	s.Unbind(req.UID)

	return nil
}

// GetIP 获取客户端IP地址
func (e *endpoint) GetIP(ctx context.Context, req *protocol.GetIPRequest, reply *protocol.GetIPReply) error {
	s, err := e.provider.Session(req.Kind, req.Target)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			reply.Code = code.NotFoundSession
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
		return err
	}

	ip, err := s.RemoteIP()
	if err != nil {
		reply.Code = code.Internal
		return err
	}

	reply.IP = ip

	return nil
}

// Push 推送消息给连接
func (e *endpoint) Push(ctx context.Context, req *protocol.PushRequest, reply *protocol.PushReply) error {
	msg, err := packet.Pack(&packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		reply.Code = code.Internal
		return err
	}

	if err = e.provider.Push(req.Kind, req.Target, msg); err != nil {
		switch err {
		case session.ErrNotFoundSession:
			reply.Code = code.NotFoundSession
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
		return err
	}

	return nil
}

// Multicast 推送组播消息
func (e *endpoint) Multicast(ctx context.Context, req *protocol.MulticastRequest, reply *protocol.MulticastReply) error {
	msg, err := packet.Pack(&packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		reply.Code = code.Internal
		return err
	}

	total, err := e.provider.Multicast(req.Kind, req.Targets, msg)
	if err != nil {
		switch err {
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
		return err
	}

	reply.Total = total

	return nil
}

// Broadcast 推送广播消息
func (e *endpoint) Broadcast(ctx context.Context, req *protocol.BroadcastRequest, reply *protocol.BroadcastReply) error {
	msg, err := packet.Pack(&packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		reply.Code = code.Internal
		return err
	}

	total, err := e.provider.Broadcast(req.Kind, msg)
	if err != nil {
		switch err {
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
		return err
	}

	reply.Total = total

	return nil
}

// Disconnect 断开连接
func (e *endpoint) Disconnect(ctx context.Context, req *protocol.DisconnectRequest, reply *protocol.DisconnectReply) error {
	s, err := e.provider.Session(req.Kind, req.Target)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			reply.Code = code.NotFoundSession
		case session.ErrInvalidSessionKind:
			reply.Code = code.InvalidArgument
		default:
			reply.Code = code.Internal
		}
		return err
	}

	if err = s.Close(req.IsForce); err != nil {
		reply.Code = code.Internal
		return err
	}

	return nil
}
