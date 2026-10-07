package node

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/packet"
)

// provider is a service provider.
type provider struct {
	node *Node
}

// Trigger triggers an event for the given gateway ID, connection ID, user ID and event type. It
// returns [errors.ErrNodeShutdown] when the node has been shut down, otherwise the trigger error.
func (p *provider) Trigger(ctx context.Context, gid string, cid, uid int64, event cluster.Event) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.node.trigger.trigger(event, gid, cid, uid)
	}
}

// Deliver delivers a message buffer to the given gateway, node, connection and user. It returns the
// error reported when the delivery fails, and [errors.ErrNodeShutdown] when the node has been shut
// down.
func (p *provider) Deliver(ctx context.Context, gid, nid string, cid, uid int64, buf buffer.Buffer) error {
	if p.node.isShut() {
		buf.Release()
		return errors.ErrNodeShutdown
	}

	route, seq, message, err := packet.UnpackMessage(buf)
	if err != nil {
		buf.Release()
		return err
	}

	stateful, ok := p.node.router.CheckRouteStateful(route)
	if !ok && !p.node.router.HasDefaultRouteHandler() {
		buf.Release()
		log.Warnf("message routing does not register handler function, route: %v", route)
		return nil
	}

	if stateful {
		if uid == 0 {
			buf.Release()
			return errors.ErrInvalidArgument
		}

		if _, ok, err = p.node.proxy.AskNode(ctx, uid, p.node.opts.name, p.node.opts.id); err != nil {
			buf.Release()
			return err
		}

		if !ok {
			buf.Release()
			return errors.ErrNotFoundSession
		}
	}

	return p.node.router.deliver(gid, nid, "", cid, uid, seq, route, message)
}

// GetState returns the current node state. The error is always nil.
func (p *provider) GetState() (cluster.State, error) {
	return p.node.getState(), nil
}

// SetState sets the node state. It returns the error reported while setting the state.
func (p *provider) SetState(state cluster.State) error {
	return p.node.setState(state)
}
