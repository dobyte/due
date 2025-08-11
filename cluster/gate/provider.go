package gate

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/utils/xcall"
)

type provider struct {
	gate *Gate
}

// Bind 绑定用户与网关间的关系
func (p *provider) Bind(ctx context.Context, cid, uid int64) error {
	if cid <= 0 || uid <= 0 {
		return errors.ErrInvalidArgument
	}

	err := p.gate.session.Bind(cid, uid)
	if err != nil {
		return err
	}

	err = p.gate.proxy.bindGate(ctx, cid, uid)
	if err != nil {
		_, _ = p.gate.session.Unbind(uid)
	}

	return err
}

// Unbind 解绑用户与网关间的关系
func (p *provider) Unbind(ctx context.Context, uid int64) error {
	if uid == 0 {
		return errors.ErrInvalidArgument
	}

	cid, err := p.gate.session.Unbind(uid)
	if err != nil {
		return err
	}

	return p.gate.proxy.unbindGate(ctx, cid, uid)
}

// GetIP 获取客户端IP地址
func (p *provider) GetIP(ctx context.Context, kind session.Kind, target int64) (string, error) {
	return p.gate.session.RemoteIP(kind, target)
}

// IsOnline 检测是否在线
func (p *provider) IsOnline(ctx context.Context, kind session.Kind, target int64) (bool, error) {
	return p.gate.session.Has(kind, target)
}

// Stat 统计会话总数
func (p *provider) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	return p.gate.session.Stat(kind)
}

// Disconnect 断开连接
func (p *provider) Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error {
	return p.gate.session.Close(kind, target, force)
}

// Push 发送消息
func (p *provider) Push(ctx context.Context, kind session.Kind, target int64, message []byte) error {
	err := p.gate.session.Push(kind, target, message)

	if kind == session.User && errors.Is(err, errors.ErrNotFoundSession) {
		xcall.Go(func() {
			if e := p.gate.opts.locator.UnbindGate(ctx, target, p.gate.opts.id); err != nil {
				log.Errorf("unbind gate failed, uid = %d gid = %s err = %v", target, p.gate.opts.id, e)
			}
		})
	}

	return err
}

// Multicast 推送组播消息
func (p *provider) Multicast(ctx context.Context, kind session.Kind, targets []int64, message []byte) (int64, error) {
	return p.gate.session.Multicast(kind, targets, message)
}

// Broadcast 推送广播消息
func (p *provider) Broadcast(ctx context.Context, kind session.Kind, message []byte) (int64, error) {
	return p.gate.session.Broadcast(kind, message)
}

// Publish 发布频道消息
func (p *provider) Publish(ctx context.Context, channel string, message []byte) int64 {
	return p.gate.session.Publish(channel, message)
}

// Subscribe 订阅频道
func (p *provider) Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	return p.gate.session.Subscribe(kind, targets, channel)
}

// Unsubscribe 取消订阅频道
func (p *provider) Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	return p.gate.session.Unsubscribe(kind, targets, channel)
}

// GetState 获取状态
func (p *provider) GetState() (cluster.State, error) {
	return cluster.Work, nil
}

// SetState 设置状态
func (p *provider) SetState(state cluster.State) error {
	return nil
}
