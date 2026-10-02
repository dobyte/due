package gate

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	taskpool "github.com/dobyte/due/v2/task"
)

// proxy is the gate proxy.
//
// It handles internal communication with business nodes, user location, event triggering and
// message delivery.
type proxy struct {
	gate       *Gate            // gate
	nodeLinker *link.NodeLinker // node linker
}

// newProxy creates a gate proxy and builds the node linker from the gate options.
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

// bindGate binds the user to this gate and records the binding in the locator, triggering the
// reconnect event on success.
func (p *proxy) bindGate(ctx context.Context, cid, uid int64) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	}

	if err := p.gate.opts.locator.BindGate(ctx, uid, p.gate.opts.id); err != nil {
		return err
	}

	taskpool.Add(func() { p.trigger(ctx, cluster.Reconnect, cid, uid) })

	return nil
}

// unbindGate unbinds the user from the gate.
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

// trigger triggers an event.
//
// It delivers connect, disconnect and reconnect events to the matching business node.
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

// deliver delivers a message.
//
// It unpacks the client message and delivers it to the route handler of the matching business
// node.
func (p *proxy) deliver(ctx context.Context, conn network.Conn, buf buffer.Buffer) {
	if p.gate.isShut() {
		buf.Release()
		return
	}

	route, seq, err := packet.ExtractRouteSeq(buf)
	if err != nil {
		buf.Release()
		log.Errorf("unpack message failed: %v", err)
		return
	}

	cid, uid := conn.ID(), conn.UID()

	if err = p.nodeLinker.Deliver(ctx, &link.DeliverArgs{
		CID:    cid,
		UID:    uid,
		Route:  route,
		Buffer: buf,
	}); err != nil {
		switch {
		case errors.Is(err, errors.ErrNotFoundRoute), errors.Is(err, errors.ErrNotFoundEndpoint):
			log.Warnf("deliver message failed, cid: %d uid: %d route: %d seq: %d err: %v", cid, uid, route, seq, err)
		default:
			log.Errorf("deliver message failed, cid: %d uid: %d route: %d seq: %d, err: %v", cid, uid, route, seq, err)
		}
	} else {
		if mode.IsDebugMode() {
			log.Debugf("deliver message success, cid: %d uid: %d route: %d seq: %d", cid, uid, route, seq)
		}
	}
}

// watch starts watching user location changes and cluster instance changes.
func (p *proxy) watch() {
	p.nodeLinker.WatchUserLocate()

	p.nodeLinker.WatchClusterInstance()
}
