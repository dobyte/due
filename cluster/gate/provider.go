package gate

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/task"
)

// provider is the gate service provider.
//
// It handles the management requests forwarded by the internal RPC link server.
type provider struct {
	gate *Gate // gate
}

// Bind binds the user to the gate.
func (p *provider) Bind(ctx context.Context, cid, uid int64) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	}

	if cid <= 0 || uid <= 0 {
		return errors.ErrInvalidArgument
	}

	// Serialize requests for the same connection, including UID changes, so an older failure
	// cannot roll back a newer request for that CID. Disconnect never takes this lock.
	unlockConn := p.gate.connBindings.lock(cid)
	err := p.bindConn(ctx, cid, uid)
	unlockConn()
	return err
}

// bindConn updates a connection's identity while the caller holds its connection binding lock.
func (p *provider) bindConn(ctx context.Context, cid, uid int64) error {
	previousUID, err := p.gate.session.UserID(cid)
	if err != nil {
		return err
	}
	if previousUID != 0 && previousUID != uid {
		if err := p.unbindPrevious(ctx, cid, previousUID); err != nil {
			return err
		}
	}
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	}

	if err := p.gate.session.Bind(cid, uid); err != nil {
		return err
	}

	// Session.Bind may synchronously close an older connection. Acquire the user lock only
	// afterwards so that the old connection's disconnect callback can finish its own cleanup.
	unlock := p.gate.bindings.lock(uid)
	if !p.gate.session.IsBound(cid, uid) {
		unlock()
		return errors.ErrNotFoundSession
	}

	if err := p.gate.proxy.bindGate(ctx, cid, uid); err != nil {
		_ = p.gate.session.UnbindConn(cid, uid)
		unlock()
		return err
	}
	bound := p.gate.session.IsBound(cid, uid)
	unlock()
	if !bound {
		return errors.ErrNotFoundSession
	}

	return nil
}

// unbindPrevious finishes the old identity's location cleanup before a connection changes UID.
// Only one user lock is held at a time, so concurrent UID swaps cannot reverse the lock order.
func (p *provider) unbindPrevious(ctx context.Context, cid, uid int64) error {
	unlock := p.gate.bindings.lock(uid)
	if !p.gate.session.IsBound(cid, uid) {
		unlock()
		return nil
	}
	if err := p.gate.proxy.unbindGate(ctx, cid, uid); err != nil {
		unlock()
		return err
	}
	err := p.gate.session.UnbindConn(cid, uid)
	unlock()
	if err != nil && !errors.Is(err, errors.ErrNotFoundSession) {
		return err
	}
	return nil
}

// Unbind unbinds the user from the gate.
func (p *provider) Unbind(ctx context.Context, uid int64) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	}

	if uid == 0 {
		return errors.ErrInvalidArgument
	}

	unlock := p.gate.bindings.lock(uid)

	cid, err := p.gate.session.Unbind(uid)
	if err != nil {
		unlock()
		return err
	}

	err = p.gate.proxy.unbindGate(ctx, cid, uid)
	unlock()
	return err
}

// GetIP returns the client IP address.
func (p *provider) GetIP(ctx context.Context, kind session.Kind, target int64) (string, error) {
	if p.gate.isShut() {
		return "", errors.ErrGateShutdown
	} else {
		return p.gate.session.RemoteIP(kind, target)
	}
}

// IsOnline reports whether the session is online.
func (p *provider) IsOnline(ctx context.Context, kind session.Kind, target int64) (bool, error) {
	if p.gate.isShut() {
		return false, errors.ErrGateShutdown
	} else {
		return p.gate.session.Has(kind, target)
	}
}

// Stat returns the total number of sessions.
func (p *provider) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	if p.gate.isShut() {
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.Stat(kind)
	}
}

// Disconnect disconnects the session.
func (p *provider) Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	} else {
		return p.gate.session.Close(kind, target, force)
	}
}

// Push pushes a message.
//
// When the pushed user does not exist (the session is not found), it asynchronously unbinds the
// user's stale gate binding from the locator.
func (p *provider) Push(ctx context.Context, kind session.Kind, target int64, disconnect bool, buf buffer.Buffer) error {
	if p.gate.isShut() {
		buf.Release()
		return errors.ErrGateShutdown
	}

	if err := p.gate.session.Push(kind, target, disconnect, buf); err != nil {
		if kind == session.User && errors.Is(err, errors.ErrNotFoundSession) {
			task.Add(func() {
				if e := p.gate.opts.locator.UnbindGate(ctx, target, p.gate.opts.id); e != nil {
					log.Errorf("unbind gate failed, uid = %d gid = %s err = %v", target, p.gate.opts.id, e)
				}
			})
		}

		return err
	}

	return nil
}

// Multicast pushes a multicast message.
func (p *provider) Multicast(ctx context.Context, kind session.Kind, targets []int64, disconnect bool, buf buffer.Buffer) (int64, error) {
	if p.gate.isShut() {
		buf.Release()
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.Multicast(kind, targets, disconnect, buf)
	}
}

// Broadcast pushes a broadcast message.
func (p *provider) Broadcast(ctx context.Context, kind session.Kind, disconnect bool, buf buffer.Buffer) (int64, error) {
	if p.gate.isShut() {
		buf.Release()
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.Broadcast(kind, disconnect, buf)
	}
}

// Publish publishes a channel message.
func (p *provider) Publish(ctx context.Context, channel string, disconnect bool, buf buffer.Buffer) (int64, error) {
	if p.gate.isShut() {
		buf.Release()
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.Publish(channel, disconnect, buf)
	}
}

// Subscribe subscribes to a channel.
func (p *provider) Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	} else {
		return p.gate.session.Subscribe(kind, targets, channel)
	}
}

// Unsubscribe unsubscribes from a channel.
func (p *provider) Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	} else {
		return p.gate.session.Unsubscribe(kind, targets, channel)
	}
}

// GetState returns the current state.
func (p *provider) GetState() (cluster.State, error) {
	return p.gate.getState(), nil
}

// SetState sets the state.
func (p *provider) SetState(state cluster.State) error {
	return p.gate.setState(state)
}
