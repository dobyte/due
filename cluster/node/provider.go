package node

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/transport"
)

type provider struct {
	node *Node
}

// LocateNode 定位用户所在节点
func (p *provider) LocateNode(ctx context.Context, uid int64) (nid string, miss bool, err error) {
	nid, err = p.node.proxy.LocateNode(ctx, uid)
	if err != nil && err != ErrNotFoundUserSource {
		return
	}

	if nid != p.node.opts.id {
		err = ErrNotFoundUserSource
	}

	miss = err == ErrNotFoundUserSource

	return
}

// CheckRouteStateful 检测某个路由是否为有状态路由
func (p *provider) CheckRouteStateful(route int32) (bool, bool) {
	return p.node.checkRouteStateful(route)
}

// Trigger 触发事件
func (p *provider) Trigger(event cluster.Event, gid string, uid int64) {
	p.node.triggerEvent(event, gid, uid)
}

// Deliver 投递消息
func (p *provider) Deliver(gid, nid string, cid, uid int64, message *transport.Message) {
	p.node.deliverMessage(gid, nid, cid, uid, &Message{
		Seq:   message.Seq,
		Route: message.Route,
		Data:  message.Buffer,
	})
}
