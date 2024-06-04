package gate

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/internal/link/types"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	"github.com/dobyte/due/v2/packet"
)

type proxy struct {
	gate       *Gate            // 网关服
	nodeLinker *link.NodeLinker // 节点链接器
}

func newProxy(gate *Gate) *proxy {
	return &proxy{gate: gate, nodeLinker: link.NewNodeLinker(gate.ctx, &link.Options{
		InsID:    gate.opts.id,
		InsKind:  cluster.Gate,
		Locator:  gate.opts.locator,
		Registry: gate.opts.registry,
	})}
}

// 绑定用户与网关间的关系
func (p *proxy) bindGate(ctx context.Context, cid, uid int64) error {
	err := p.gate.opts.locator.BindGate(ctx, uid, p.gate.opts.id)
	if err != nil {
		return err
	}

	p.trigger(ctx, cluster.Reconnect, cid, uid)

	return nil
}

// 解绑用户与网关间的关系
func (p *proxy) unbindGate(ctx context.Context, cid, uid int64) error {
	err := p.gate.opts.locator.UnbindGate(ctx, uid, p.gate.opts.id)
	if err != nil {
		log.Errorf("user unbind failed, gid: %d, cid: %d, uid: %d, err: %v", p.gate.opts.id, cid, uid, err)
	}

	return err
}

// 触发事件
func (p *proxy) trigger(ctx context.Context, event cluster.Event, cid, uid int64) {
	if mode.IsDebugMode() {
		log.Debugf("trigger event, event: %v cid: %d uid: %d", event.String(), cid, uid)
	}

	if err := p.nodeLinker.Trigger(ctx, &types.TriggerArgs{
		Event: event,
		CID:   cid,
		UID:   uid,
		Async: true,
	}); err != nil && !errors.Is(err, errors.ErrNotFoundEvent) && !errors.Is(err, errors.ErrNotFoundUserLocation) {
		log.Warnf("trigger event failed, cid: %d, uid: %d, event: %v, err: %v", cid, uid, event.String(), err)
	}
}

// 投递消息
func (p *proxy) deliver(ctx context.Context, cid, uid int64, message []byte) {
	msg, err := packet.UnpackMessage(message)
	if err != nil {
		log.Errorf("unpack message failed: %v", err)
		return
	}

	if mode.IsDebugMode() {
		log.Debugf("deliver message, cid: %d uid: %d route: %d buffer: %s", cid, uid, msg.Route, string(msg.Buffer))
	}

	if err = p.nodeLinker.Deliver(ctx, &types.DeliverArgs{
		CID:     cid,
		UID:     uid,
		Route:   msg.Route,
		Message: message,
	}); err != nil {
		log.Errorf("deliver message failed, cid = %d uid = %d err = %v", cid, uid, err)
	}
}
