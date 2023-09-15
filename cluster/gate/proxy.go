package gate

import (
	"context"
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/internal/link"
	"github.com/symsimmy/due/packet"

	"github.com/symsimmy/due/log"
)

var (
	ErrInvalidGID         = link.ErrInvalidGID
	ErrInvalidNID         = link.ErrInvalidNID
	ErrInvalidMessage     = link.ErrInvalidMessage
	ErrInvalidArgument    = link.ErrInvalidArgument
	ErrInvalidSessionKind = link.ErrInvalidSessionKind
	ErrNotFoundUserSource = link.ErrNotFoundUserSource
	ErrReceiveTargetEmpty = link.ErrReceiveTargetEmpty
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
	err := p.gate.opts.locator.Set(ctx, uid, cluster.Gate, p.gate.opts.id)
	if err != nil {
		return err
	}

	p.trigger(ctx, cluster.Reconnect, cid, uid)

	return nil
}

// 解绑用户与网关间的关系
func (p *proxy) unbindGate(ctx context.Context, cid, uid int64) error {
	err := p.gate.opts.locator.Rem(ctx, uid, cluster.Gate, p.gate.opts.id)
	if err != nil {
		log.Errorf("user unbind failed, gid: %d, cid: %d, uid: %d, err: %v", p.gate.opts.id, cid, uid, err)
	}

	return err
}

// 触发事件
func (p *proxy) trigger(ctx context.Context, event cluster.Event, cid, uid int64) {
	if err := p.link.Trigger(ctx, &link.TriggerArgs{
		Event: event,
		CID:   cid,
		UID:   uid,
	}); err != nil {
		log.Warnf("trigger event failed, gid: %s, cid: %d, uid: %d, event: %v, err: %v", p.gate.opts.id, cid, uid, event, err)
	}
}

// 投递消息
func (p *proxy) deliver(ctx context.Context, cid, uid int64, data []byte) {
	message, err := packet.Unpack(data)
	if err != nil {
		log.Errorf("unpack data to struct failed: %v", err)
		return
	}

	if err = p.link.Deliver(ctx, &link.DeliverArgs{
		CID:     cid,
		UID:     uid,
		Message: message,
	}); err != nil {
		log.Errorf("deliver message failed: %v", err)
	}
}

// 启动监听
func (p *proxy) watch(ctx context.Context) {
	p.link.WatchUserLocate(ctx, cluster.Node)

	p.link.WatchServiceInstance(ctx, cluster.Node)
}
