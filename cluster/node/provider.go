package node

import (
	"context"
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
func (p *provider) Trigger(ctx context.Context, args *transport.TriggerArgs) (bool, error) {
	if args.UID <= 0 {
		return false, ErrInvalidArgument
	}

	_, miss, err := p.LocateNode(ctx, args.UID)
	if err != nil {
		return miss, err
	}

	p.node.trigger(args.Event, args.GID, args.UID)

	return false, nil
}

// Deliver 投递消息
func (p *provider) Deliver(ctx context.Context, args *transport.DeliverArgs) (bool, error) {
	stateful, ok := p.CheckRouteStateful(args.Message.Route)
	if !ok {
		return false, nil
	}

	if stateful {
		if args.UID <= 0 {
			return false, ErrInvalidArgument
		}

		_, miss, err := p.LocateNode(ctx, args.UID)
		if err != nil {
			return miss, err
		}
	}

	req := p.node.reqPool.Get().(*Request)
	req.gid = args.GID
	req.nid = args.NID
	req.cid = args.CID
	req.uid = args.UID
	req.message.Seq = args.Message.Seq
	req.message.Route = args.Message.Route
	req.message.Data = args.Message.Buffer
	p.node.chRequest <- req

	return false, nil
}
