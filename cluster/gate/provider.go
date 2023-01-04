package gate

import (
	"context"
	"github.com/dobyte/due/packet"
	"github.com/dobyte/due/session"
)

type provider struct {
	gate *Gate
}

// Bind 绑定用户与网关间的关系
func (p *provider) Bind(ctx context.Context, cid, uid int64) error {
	if cid <= 0 || uid <= 0 {
		return ErrInvalidArgument
	}

	s, err := p.gate.group.GetSession(session.Conn, cid)
	if err != nil {
		return err
	}

	err = p.gate.proxy.bindGate(ctx, uid)
	if err != nil {
		return err
	}

	s.Bind(uid)

	return nil
}

// Unbind 解绑用户与网关间的关系
func (p *provider) Unbind(ctx context.Context, uid int64) error {
	if uid <= 0 {
		return ErrInvalidArgument
	}

	s, err := p.gate.group.GetSession(session.User, uid)
	if err != nil {
		return err
	}

	err = p.gate.proxy.unbindGate(ctx, uid)
	if err != nil {
		return err
	}

	s.Unbind(uid)

	return nil
}

// GetIP 获取客户端IP地址
func (p *provider) GetIP(kind session.Kind, target int64) (string, error) {
	s, err := p.gate.group.GetSession(kind, target)
	if err != nil {
		return "", err
	}

	return s.RemoteIP()
}

// Push 发送消息
func (p *provider) Push(kind session.Kind, target int64, message *packet.Message) error {
	msg, err := packet.Pack(message)
	if err != nil {
		return err
	}

	return p.gate.group.Push(kind, target, msg)
}

// Multicast 推送组播消息
func (p *provider) Multicast(kind session.Kind, targets []int64, message *packet.Message) (int64, error) {
	msg, err := packet.Pack(message)
	if err != nil {
		return 0, err
	}

	total, err := p.gate.group.Multicast(kind, targets, msg)

	return int64(total), err
}

// Broadcast 推送广播消息
func (p *provider) Broadcast(kind session.Kind, message *packet.Message) (int64, error) {
	msg, err := packet.Pack(message)
	if err != nil {
		return 0, err
	}

	total, err := p.gate.group.Broadcast(kind, msg)

	return int64(total), err
}

// Disconnect 断开连接
func (p *provider) Disconnect(kind session.Kind, target int64, isForce bool) error {
	s, err := p.gate.group.GetSession(kind, target)
	if err != nil {
		return err
	}

	return s.Close(isForce)
}
