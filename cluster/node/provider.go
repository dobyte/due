package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/transport"
)

type provider struct {
	node *Node
}

// Trigger 触发事件
func (p *provider) Trigger(ctx context.Context, args *transport.TriggerArgs) (bool, error) {
	evt := cluster.Event(args.Event)

	switch evt {
	case cluster.Connect:
		// ignore
	case cluster.Reconnect:
		if args.UID <= 0 {
			return false, errors.ErrInvalidArgument
		}

		_, ok, err := p.node.proxy.AskNode(ctx, args.UID, p.node.opts.name, p.node.opts.id)
		if err != nil {
			return false, err
		}

		if !ok {
			return true, errors.ErrNotFoundUserLocation
		}
	case cluster.Disconnect:
		if args.UID > 0 {
			_, ok, err := p.node.proxy.AskNode(ctx, args.UID, p.node.opts.name, p.node.opts.id)
			if err != nil {
				return false, err
			}

			if !ok {
				return true, errors.ErrNotFoundUserLocation
			}
		}
	}

	p.node.trigger.trigger(evt, args.GID, args.CID, args.UID)

	return false, nil
}

// Deliver 投递消息
func (p *provider) Deliver(ctx context.Context, gid, nid string, cid, uid int64, message []byte) error {
	msg, err := packet.UnpackMessage(message)
	if err != nil {
		return err
	}

	stateful, ok := p.node.router.CheckRouteStateful(msg.Route)
	if !ok {
		if ok = p.node.router.HasDefaultRouteHandler(); !ok {
			return nil
		}
	}

	if stateful {
		if uid <= 0 {
			return errors.ErrInvalidArgument
		}

		_, ok, err = p.node.proxy.AskNode(ctx, uid, p.node.opts.name, p.node.opts.id)
		if err != nil {
			return err
		}

		if !ok {
			return errors.ErrNotFoundSession
		}
	}

	p.node.router.deliver(gid, nid, cid, uid, msg.Seq, msg.Route, msg.Buffer)

	return nil
}
