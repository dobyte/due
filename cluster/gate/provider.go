package gate

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/task"
)

// provider 网关服务提供者
// 用于处理内网RPC链接服务器转发的各项管理请求
type provider struct {
	gate *Gate
}

// Bind 绑定用户与网关间的关系
// @param ctx context.Context 上下文
// @param cid int64 连接ID
// @param uid int64 用户ID
// @return @1 error 错误信息
func (p *provider) Bind(ctx context.Context, cid, uid int64) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	}

	if cid <= 0 || uid <= 0 {
		return errors.ErrInvalidArgument
	}

	if err := p.gate.session.Bind(cid, uid); err != nil {
		return err
	}

	if err := p.gate.proxy.bindGate(ctx, cid, uid); err != nil {
		_, _ = p.gate.session.Unbind(uid)
		return err
	}

	return nil
}

// Unbind 解绑用户与网关间的关系
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @return @1 error 错误信息
func (p *provider) Unbind(ctx context.Context, uid int64) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	}

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
// @param ctx context.Context 上下文
// @param kind session.Kind 会话类型
// @param target int64 会话目标
// @return @1 string 客户端IP地址
// @return @2 error 错误信息
func (p *provider) GetIP(ctx context.Context, kind session.Kind, target int64) (string, error) {
	if p.gate.isShut() {
		return "", errors.ErrGateShutdown
	} else {
		return p.gate.session.RemoteIP(kind, target)
	}
}

// IsOnline 检测是否在线
// @param ctx context.Context 上下文
// @param kind session.Kind 会话类型
// @param target int64 会话目标
// @return @1 bool 是否在线
// @return @2 error 错误信息
func (p *provider) IsOnline(ctx context.Context, kind session.Kind, target int64) (bool, error) {
	if p.gate.isShut() {
		return false, errors.ErrGateShutdown
	} else {
		return p.gate.session.Has(kind, target)
	}
}

// Stat 统计会话总数
// @param ctx context.Context 上下文
// @param kind session.Kind 会话类型
// @return @1 int64 会话总数
// @return @2 error 错误信息
func (p *provider) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	if p.gate.isShut() {
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.Stat(kind)
	}
}

// Disconnect 断开连接
// @param ctx context.Context 上下文
// @param kind session.Kind 会话类型
// @param target int64 会话目标
// @param force bool 是否强制断开
// @return @1 error 错误信息
func (p *provider) Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	} else {
		return p.gate.session.Close(kind, target, force)
	}
}

// Push 发送消息
// 当推送用户不存在（会话未找到）时，异步解绑该用户在定位器上的失效网关绑定
// @param ctx context.Context 上下文
// @param kind session.Kind 会话类型
// @param target int64 会话目标
// @param disconnect bool 是否在推送后断开连接
// @param message []byte 消息内容
// @return @1 error 错误信息
func (p *provider) Push(ctx context.Context, kind session.Kind, target int64, disconnect bool, message []byte) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	} else {
		if err := p.gate.session.Push(kind, target, disconnect, message); err != nil {
			if kind == session.User && errors.Is(err, errors.ErrNotFoundSession) {
				task.Add(func() {
					if e := p.gate.opts.locator.UnbindGate(ctx, target, p.gate.opts.id); e != nil {
						log.Errorf("unbind gate failed, uid = %d gid = %s err = %v", target, p.gate.opts.id, e)
					}
				})
			}

			return err
		}

		return nil
	}
}

// Multicast 推送组播消息
// @param ctx context.Context 上下文
// @param kind session.Kind 会话类型
// @param targets []int64 会话目标列表
// @param disconnect bool 是否在推送后断开连接
// @param message []byte 消息内容
// @return @1 int64 推送成功的目标数
// @return @2 error 错误信息
func (p *provider) Multicast(ctx context.Context, kind session.Kind, targets []int64, disconnect bool, message []byte) (int64, error) {
	if p.gate.isShut() {
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.Multicast(kind, targets, disconnect, message)
	}
}

// Broadcast 推送广播消息
// @param ctx context.Context 上下文
// @param kind session.Kind 会话类型
// @param disconnect bool 是否在推送后断开连接
// @param message []byte 消息内容
// @return @1 int64 推送成功的目标数
// @return @2 error 错误信息
func (p *provider) Broadcast(ctx context.Context, kind session.Kind, disconnect bool, message []byte) (int64, error) {
	if p.gate.isShut() {
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.Broadcast(kind, disconnect, message)
	}
}

// Publish 发布频道消息
// @param ctx context.Context 上下文
// @param channel string 频道名称
// @param disconnect bool 是否在推送后断开连接
// @param message []byte 消息内容
// @return @1 int64 推送成功的目标数
// @return @2 error 错误信息
func (p *provider) Publish(ctx context.Context, channel string, disconnect bool, message []byte) (int64, error) {
	if p.gate.isShut() {
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.Publish(channel, disconnect, message)
	}
}

// Subscribe 订阅频道
// @param ctx context.Context 上下文
// @param kind session.Kind 会话类型
// @param targets []int64 会话目标列表
// @param channel string 频道名称
// @return @1 error 错误信息
func (p *provider) Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	} else {
		return p.gate.session.Subscribe(kind, targets, channel)
	}
}

// Unsubscribe 取消订阅频道
// @param ctx context.Context 上下文
// @param kind session.Kind 会话类型
// @param targets []int64 会话目标列表
// @param channel string 频道名称
// @return @1 error 错误信息
func (p *provider) Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	} else {
		return p.gate.session.Unsubscribe(kind, targets, channel)
	}
}

// GetState 获取状态
// @return @1 cluster.State 当前状态
// @return @2 error 错误信息
func (p *provider) GetState() (cluster.State, error) {
	return p.gate.getState(), nil
}

// SetState 设置状态
// @param state cluster.State 目标状态
// @return @1 error 错误信息
func (p *provider) SetState(state cluster.State) error {
	return p.gate.setState(state)
}
