package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
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
func (p *provider) Deliver(ctx context.Context, args *transport.DeliverArgs) (bool, error) {
	stateful, ok := p.node.router.CheckRouteStateful(args.Message.Route)
	if !ok {
		if ok = p.node.router.HasDefaultRouteHandler(); !ok {
			return false, nil
		}
	}

	if stateful {
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
	}

	p.node.router.deliver(args.GID, args.NID, args.CID, args.UID, args.Message.Seq, args.Message.Route, args.Message.Buffer)

	return false, nil
}
