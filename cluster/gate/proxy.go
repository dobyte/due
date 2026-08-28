package gate

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	"github.com/dobyte/due/v2/packet"
)

// proxy 网关代理
// 负责与业务节点的内网通信、用户定位、事件触发与消息投递
type proxy struct {
	gate       *Gate            // 网关服
	nodeLinker *link.NodeLinker // 节点链接器
}

// 创建网关代理
// 基于网关配置构造节点链接器
// @param gate *Gate 网关组件
// @return @1 *proxy 网关代理
func newProxy(gate *Gate) *proxy {
	return &proxy{gate: gate, nodeLinker: link.NewNodeLinker(gate.ctx, &link.Options{
		ID:                  gate.opts.id,
		Kind:                cluster.Gate,
		Locator:             gate.opts.locator,
		Registry:            gate.opts.registry,
		Dispatch:            gate.opts.dispatch,
		ConnNum:             gate.opts.linker.connNum,
		CallTimeout:         gate.opts.linker.callTimeout,
		DialTimeout:         gate.opts.linker.dialTimeout,
		DialRetryTimes:      gate.opts.linker.dialRetryTimes,
		FaultRecoveryTime:   gate.opts.linker.faultRecoveryTime,
		CommandQueueSize:    gate.opts.linker.commandQueueSize,
		CommandWriteTimeout: gate.opts.linker.commandWriteTimeout,
	})}
}

// 绑定用户与网关间的关系
// 将用户绑定到本网关并记录到定位器，绑定成功后触发重连事件
// @param ctx context.Context 上下文
// @param cid int64 连接ID
// @param uid int64 用户ID
// @return @1 error 错误信息
func (p *proxy) bindGate(ctx context.Context, cid, uid int64) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	}

	if err := p.gate.opts.locator.BindGate(ctx, uid, p.gate.opts.id); err != nil {
		return err
	}

	p.trigger(ctx, cluster.Reconnect, cid, uid)

	return nil
}

// 解绑用户与网关间的关系
// @param ctx context.Context 上下文
// @param cid int64 连接ID
// @param uid int64 用户ID
// @return @1 error 错误信息
func (p *proxy) unbindGate(ctx context.Context, cid, uid int64) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	}

	if err := p.gate.opts.locator.UnbindGate(ctx, uid, p.gate.opts.id); err != nil {
		if mode.IsDebugMode() {
			log.Debugf("user unbind failed, gid: %s, cid: %d, uid: %d, err: %v", p.gate.opts.id, cid, uid, err)
		}

		return err
	} else {
		return nil
	}
}

// 触发事件
// 将连接/断开/重连等事件投递到对应业务节点
// @param ctx context.Context 上下文
// @param event cluster.Event 事件类型
// @param cid int64 连接ID
// @param uid int64 用户ID
func (p *proxy) trigger(ctx context.Context, event cluster.Event, cid, uid int64) {
	if p.gate.isShut() {
		return
	}

	if mode.IsDebugMode() {
		log.Debugf("trigger event, event: %v cid: %d uid: %d", event.String(), cid, uid)
	}

	if err := p.nodeLinker.Trigger(ctx, &link.TriggerArgs{
		Event: event,
		CID:   cid,
		UID:   uid,
	}); err != nil {
		switch {
		case errors.Is(err, errors.ErrNotFoundEvent), errors.Is(err, errors.ErrNotFoundUserLocation):
			log.Warnf("trigger event failed, cid: %d, uid: %d, event: %v, err: %v", cid, uid, event.String(), err)
		default:
			log.Errorf("trigger event failed, cid: %d, uid: %d, event: %v, err: %v", cid, uid, event.String(), err)
		}
	}
}

// 投递消息
// 解包客户端消息并投递到对应业务节点的路由处理器
// @param ctx context.Context 上下文
// @param cid int64 连接ID
// @param uid int64 用户ID
// @param data []byte 原始消息内容
func (p *proxy) deliver(ctx context.Context, cid, uid int64, data []byte) {
	if p.gate.isShut() {
		return
	}

	message, err := packet.UnpackMessage(data)
	if err != nil {
		log.Errorf("unpack message failed: %v", err)
		return
	}

	if err = p.nodeLinker.Deliver(ctx, &link.DeliverArgs{
		CID:    cid,
		UID:    uid,
		Route:  message.Route,
		Buffer: data,
	}); err != nil {
		switch {
		case errors.Is(err, errors.ErrNotFoundRoute), errors.Is(err, errors.ErrNotFoundEndpoint):
			log.Warnf("deliver message failed, cid: %d uid: %d seq: %d route: %d err: %v", cid, uid, message.Seq, message.Route, err)
		default:
			log.Errorf("deliver message failed, cid: %d uid: %d seq: %d route: %d err: %v", cid, uid, message.Seq, message.Route, err)
		}
	} else {
		if mode.IsDebugMode() {
			log.Debugf("deliver message success, cid: %d uid: %d seq: %d route: %d", cid, uid, message.Seq, message.Route)
		}
	}
}

// 开始监听
// 监听用户定位变化与集群实例变化
func (p *proxy) watch() {
	p.nodeLinker.WatchUserLocate()

	p.nodeLinker.WatchClusterInstance()
}
