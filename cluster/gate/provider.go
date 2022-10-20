package gate

import (
	"context"
	"github.com/dobyte/due/session"
)

type provider struct {
	gate *Gate
}

// Session 获取会话
func (p *provider) Session(kind session.Kind, target int64) (*session.Session, error) {
	return p.gate.group.GetSession(kind, target)
}

// Bind 绑定用户与网关间的关系
func (p *provider) Bind(ctx context.Context, uid int64) error {
	return p.gate.proxy.bindGate(ctx, uid)
}

// Unbind 解绑用户与网关间的关系
func (p *provider) Unbind(ctx context.Context, uid int64) error {
	return p.gate.proxy.unbindGate(ctx, uid)
}

// Push 发送消息
func (p *provider) Push(kind session.Kind, target int64, msg []byte, msgType ...int) error {
	return p.gate.group.Push(kind, target, msg, msgType...)
}

// Multicast 推送组播消息
func (p *provider) Multicast(kind session.Kind, targets []int64, msg []byte, msgType ...int) (n int, err error) {
	return p.gate.group.Multicast(kind, targets, msg, msgType...)
}

// Broadcast 推送广播消息
func (p *provider) Broadcast(kind session.Kind, msg []byte, msgType ...int) (n int, err error) {
	return p.gate.group.Broadcast(kind, msg, msgType...)
}
