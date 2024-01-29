package gate

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/packet"
)

type proxy struct {
	gate *Gate      // 网关服
	link *link.Link // 连接
}

func newProxy(gate *Gate) *proxy {
	return &proxy{gate: gate, link: link.NewLink(&link.Options{
		GID:         gate.opts.id,
		Locator:     gate.opts.locator,
		Registry:    gate.opts.registry,
		Transporter: gate.opts.transporter,
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
	if err := p.link.Trigger(ctx, &link.TriggerArgs{
		Event: int(event),
		CID:   cid,
		UID:   uid,
	}); err != nil && err != errors.ErrNotFoundEvent && err != errors.ErrNotFoundUserLocation {
		log.Warnf("trigger event failed, cid: %d, uid: %d, event: %v, err: %v", cid, uid, event.String(), err)
	}
}

// 投递消息
func (p *proxy) deliver(ctx context.Context, cid, uid int64, data []byte) {
	message, err := packet.UnpackMessage(data)
	if err != nil {
		log.Errorf("unpack data to struct failed: %v", err)
		return
	}

	log.Debugf("deliver message: cid: %d uid: %d route: %d buffer: %s", cid, uid, message.Route, string(message.Buffer))

	if err = p.link.Deliver(ctx, &link.DeliverArgs{
		CID:     cid,
		UID:     uid,
		Message: message,
	}); err != nil {
		log.Errorf("deliver message failed, cid = %d uid = %d route = %d err = %v", cid, uid, message.Route, err)
	}
}

// 启动监听
func (p *proxy) watch(ctx context.Context) {
	p.link.WatchUserLocate(ctx, cluster.Node.String())

	p.link.WatchServiceInstance(ctx, cluster.Node.String())
}
