package gate

import (
	"context"
	"github.com/dobyte/due/transport/kitex/v2/internal/protocol/gate"
	inner "github.com/dobyte/due/transport/kitex/v2/internal/protocol/gate/gate"
	"github.com/dobyte/due/transport/kitex/v2/internal/server"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
)

func NewServer(provider transport.GateProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	err = s.RegisterService(inner.NewServiceInfo(), &endpoint{provider: provider})
	if err != nil {
		return nil, err
	}

	return s, nil
}

type endpoint struct {
	provider transport.GateProvider
}

// Bind 将用户与当前网关进行绑定
func (e *endpoint) Bind(ctx context.Context, req *gate.BindRequest) (*gate.BindResponse, error) {
	err := e.provider.Bind(ctx, req.CID, req.UID)
	if err != nil {
		return nil, err
	}

	return &gate.BindResponse{}, nil
}

// Unbind 将用户与当前网关进行解绑
func (e *endpoint) Unbind(ctx context.Context, req *gate.UnbindRequest) (*gate.UnbindResponse, error) {
	err := e.provider.Unbind(ctx, req.UID)
	if err != nil {
		return nil, err
	}

	return &gate.UnbindResponse{}, nil
}

// GetIP 获取客户端IP地址
func (e *endpoint) GetIP(ctx context.Context, req *gate.GetIPRequest) (*gate.GetIPResponse, error) {
	ip, err := e.provider.GetIP(ctx, session.Kind(req.Kind), req.Target)
	if err != nil {
		return nil, err
	}

	return &gate.GetIPResponse{IP: ip}, nil
}

// Stat 统计会话总数
func (e *endpoint) Stat(ctx context.Context, req *gate.StatRequest) (*gate.StatResponse, error) {
	total, err := e.provider.Stat(ctx, session.Kind(req.Kind))
	if err != nil {
		return nil, err
	}

	return &gate.StatResponse{Total: total}, nil
}

// Disconnect 断开连接
func (e *endpoint) Disconnect(ctx context.Context, req *gate.DisconnectRequest) (*gate.DisconnectResponse, error) {
	err := e.provider.Disconnect(ctx, session.Kind(req.Kind), req.Target, req.IsForce)
	if err != nil {
		return nil, err
	}

	return &gate.DisconnectResponse{}, nil
}

// Push 推送消息给连接
func (e *endpoint) Push(ctx context.Context, req *gate.PushRequest) (*gate.PushResponse, error) {
	err := e.provider.Push(ctx, session.Kind(req.Kind), req.Target, &packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		return nil, err
	}

	return &gate.PushResponse{}, nil
}

// Multicast 推送组播消息
func (e *endpoint) Multicast(ctx context.Context, req *gate.MulticastRequest) (*gate.MulticastResponse, error) {
	total, err := e.provider.Multicast(ctx, session.Kind(req.Kind), req.Targets, &packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		return nil, err
	}

	return &gate.MulticastResponse{Total: total}, nil
}

// Broadcast 推送广播消息
func (e *endpoint) Broadcast(ctx context.Context, req *gate.BroadcastRequest) (*gate.BroadcastResponse, error) {
	total, err := e.provider.Broadcast(ctx, session.Kind(req.Kind), &packet.Message{
		Seq:    req.Message.Seq,
		Route:  req.Message.Route,
		Buffer: req.Message.Buffer,
	})
	if err != nil {
		return nil, err
	}

	return &gate.BroadcastResponse{Total: total}, nil
}
