package gate

import (
	"context"
	"errors"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/cluster/internal"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/packet"
	"github.com/dobyte/due/router"
)

var (
    ErrInvalidGID         = internal.ErrInvalidGID
    ErrInvalidNID         = internal.ErrInvalidNID
    ErrInvalidMessage     = internal.ErrInvalidMessage
    ErrInvalidArgument    = internal.ErrInvalidArgument
    ErrInvalidSessionKind = internal.ErrInvalidSessionKind
    ErrNotFoundUserSource = internal.ErrNotFoundUserSource
    ErrReceiveTargetEmpty = internal.ErrReceiveTargetEmpty
)

type proxy struct {
	gate *Gate          // 网关服
	link *internal.Link // 连接
}

func newProxy(gate *Gate) *proxy {
	return &proxy{gate: gate, link: internal.NewLink(&internal.Options{
		GID:         gate.opts.id,
		Locator:     gate.opts.locator,
		Registry:    gate.opts.registry,
		Transporter: gate.opts.transporter,
	})}
}

// 绑定用户与网关间的关系
func (p *proxy) bindGate(ctx context.Context, uid int64) error {
	err := p.gate.opts.locator.Set(ctx, uid, cluster.Gate, p.gate.opts.id)
	if err != nil {
		return err
	}

	err = p.link.Trigger(ctx, cluster.Reconnect, uid)
	if err != nil && err != ErrNotFoundUserSource && err != router.ErrNotFoundEndpoint {
		log.Errorf("trigger event failed, gid: %s, uid: %d, event: %v, err: %v", p.gate.opts.id, uid, cluster.Reconnect, err)
	}

	return nil
}

// 解绑用户与网关间的关系
func (p *proxy) unbindGate(ctx context.Context, uid int64) error {
	err := p.gate.opts.locator.Rem(ctx, uid, cluster.Gate, p.gate.opts.id)
	if err != nil {
		return err
	}

	err = p.link.Trigger(ctx, cluster.Disconnect, uid)
	if err != nil && err != ErrNotFoundUserSource && err != router.ErrNotFoundEndpoint {
		log.Errorf("trigger event failed, gid: %s, uid: %d, event: %v, err: %v", p.gate.opts.id, uid, cluster.Disconnect, err)
	}

	return nil
}

// 投递消息
func (p *proxy) deliver(ctx context.Context, cid, uid int64, message *packet.Message) error {
	return p.link.Deliver(ctx, &internal.DeliverArgs{
		CID:     cid,
		UID:     uid,
		Message: message,
	})
}

// 启动监听
func (p *proxy) watch(ctx context.Context) {
	p.link.WatchUserLocate(ctx, cluster.Node)

	p.link.WatchServiceInstance(ctx, string(cluster.Node))
}
