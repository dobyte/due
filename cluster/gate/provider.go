package gate

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
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
	connLock := p.gate.connBindings.lock(cid)
	err := p.bindConn(ctx, cid, uid)
	connLock.unlock()
	return err
}

// bindConn updates a connection's identity while the caller holds its connection binding lock.
func (p *provider) bindConn(ctx context.Context, cid, uid int64) error {
	previousUID, err := p.gate.session.uid(cid)
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

	if err := p.gate.session.bind(cid, uid); err != nil {
		return err
	}

	if !p.gate.session.isBound(cid, uid) {
		return errors.ErrNotFoundSession
	}

	// The locator call runs without the user binding lock: a slow locator must not stall the
	// disconnect cleanup of the users that hash to the same shard. The binding is reconciled
	// afterwards because a concurrent unbind may have completed while the call was in flight.
	if err := p.gate.proxy.bindGate(ctx, cid, uid); err != nil {
		p.rollbackBind(cid, uid)
		return err
	}

	if !p.gate.session.isBound(cid, uid) {
		_ = p.cleanupStaleBinding(ctx, uid)
		return errors.ErrNotFoundSession
	}

	return nil
}

// rollbackBind removes the local binding after a failed locator bind unless a newer binding for
// the same user replaced it meanwhile.
func (p *provider) rollbackBind(cid, uid int64) {
	userLock := p.gate.userBindings.lock(uid)
	if p.gate.session.isBound(cid, uid) {
		_ = p.gate.session.unbind(cid, uid)
	}
	userLock.unlock()
}

// unbindPrevious finishes the old identity's location cleanup before a connection changes UID.
// Only one user lock is held at a time, so concurrent UID swaps cannot reverse the lock order.
func (p *provider) unbindPrevious(ctx context.Context, cid, uid int64) error {
	userLock := p.gate.userBindings.lock(uid)
	bound := p.gate.session.isBound(cid, uid)
	userLock.unlock()
	if !bound {
		return nil
	}

	// Run the locator cleanup outside the user binding lock. A failure leaves the local identity
	// unchanged so the caller can retry the UID change.
	if err := p.gate.proxy.unbindGate(ctx, cid, uid); err != nil {
		return err
	}

	userLock = p.gate.userBindings.lock(uid)
	if p.gate.session.isBound(cid, uid) {
		_ = p.gate.session.unbind(cid, uid)
		userLock.unlock()
		return nil
	}
	online, _ := p.gate.session.has(cluster.User, uid)
	userLock.unlock()

	if online {
		// A newer bind replaced the old identity while the cleanup was in flight; restore the
		// locator binding it owns.
		if err := p.gate.opts.locator.BindGate(ctx, uid, p.gate.opts.id); err != nil {
			log.Warnf("restore gate binding failed, uid: %d, gid: %s, err: %v", uid, p.gate.opts.id, err)
		}
	}

	return nil
}

// cleanupStaleBinding removes a possibly stale gate binding left by an out-of-lock transition and
// restores it when the user has been bound again by the time the removal completes. The locator
// unbind is conditional on the gate ID, so a binding owned by another gate is preserved.
func (p *provider) cleanupStaleBinding(ctx context.Context, uid int64) error {
	if err := p.gate.opts.locator.UnbindGate(ctx, uid, p.gate.opts.id); err != nil {
		return err
	}

	if online, _ := p.gate.session.has(cluster.User, uid); online {
		if err := p.gate.opts.locator.BindGate(ctx, uid, p.gate.opts.id); err != nil {
			log.Warnf("restore gate binding failed, uid: %d, gid: %s, err: %v", uid, p.gate.opts.id, err)
		}
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

	userLock := p.gate.userBindings.lock(uid)
	_, err := p.gate.session.remove(uid)
	userLock.unlock()
	if err != nil {
		return err
	}

	return p.cleanupStaleBinding(ctx, uid)
}

// GetIP returns the client IP address.
func (p *provider) GetIP(ctx context.Context, kind cluster.Session, target int64) (string, error) {
	if p.gate.isShut() {
		return "", errors.ErrGateShutdown
	} else {
		return p.gate.session.remoteIP(kind, target)
	}
}

// IsOnline reports whether the session is online.
func (p *provider) IsOnline(ctx context.Context, kind cluster.Session, target int64) (bool, error) {
	if p.gate.isShut() {
		return false, errors.ErrGateShutdown
	} else {
		return p.gate.session.has(kind, target)
	}
}

// Stat returns the total number of sessions.
func (p *provider) Stat(ctx context.Context, kind cluster.Session) (int64, error) {
	if p.gate.isShut() {
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.stat(kind)
	}
}

// Disconnect disconnects the session.
func (p *provider) Disconnect(ctx context.Context, kind cluster.Session, target int64, force bool) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	} else {
		return p.gate.session.close(kind, target, force)
	}
}

// Push pushes a message.
//
// When the pushed user does not exist (the session is not found), it asynchronously unbinds the
// user's stale gate binding from the locator unless the user has been bound again meanwhile.
func (p *provider) Push(ctx context.Context, kind cluster.Session, target int64, disconnect bool, buf buffer.Buffer) error {
	if p.gate.isShut() {
		buf.Release()
		return errors.ErrGateShutdown
	}

	if err := p.gate.session.push(kind, target, disconnect, buf); err != nil {
		if kind == cluster.User && errors.Is(err, errors.ErrNotFoundSession) {
			// The cleanup outlives the request, so it uses the component context instead of the
			// caller context, bounded like the disconnect cleanup. The caller context may be
			// canceled once the request returns, and an unbounded locator call would keep the
			// user binding lock for the whole shard.
			task.Add(func() {
				userLock := p.gate.userBindings.lock(target)
				if online, _ := p.gate.session.has(cluster.User, target); !online {
					ctx, cancel := context.WithTimeout(p.gate.ctx, 3*time.Second)
					err := p.gate.opts.locator.UnbindGate(ctx, target, p.gate.opts.id)
					cancel()
					if err != nil {
						log.Errorf("unbind gate failed, uid = %d gid = %s err = %v", target, p.gate.opts.id, err)
					}
				}
				userLock.unlock()
			})
		}

		return err
	}

	return nil
}

// Multicast pushes a multicast message.
func (p *provider) Multicast(ctx context.Context, kind cluster.Session, targets []int64, disconnect bool, buf buffer.Buffer) (int64, error) {
	if p.gate.isShut() {
		buf.Release()
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.multicast(kind, targets, disconnect, buf)
	}
}

// Broadcast pushes a broadcast message.
func (p *provider) Broadcast(ctx context.Context, kind cluster.Session, disconnect bool, buf buffer.Buffer) (int64, error) {
	if p.gate.isShut() {
		buf.Release()
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.broadcast(kind, disconnect, buf)
	}
}

// Publish publishes a channel message.
func (p *provider) Publish(ctx context.Context, channel string, disconnect bool, buf buffer.Buffer) (int64, error) {
	if p.gate.isShut() {
		buf.Release()
		return 0, errors.ErrGateShutdown
	} else {
		return p.gate.session.publish(channel, disconnect, buf)
	}
}

// Subscribe subscribes to a channel.
func (p *provider) Subscribe(ctx context.Context, kind cluster.Session, targets []int64, channel string) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	} else {
		return p.gate.session.subscribe(kind, targets, channel)
	}
}

// Unsubscribe unsubscribes from a channel.
func (p *provider) Unsubscribe(ctx context.Context, kind cluster.Session, targets []int64, channel string) error {
	if p.gate.isShut() {
		return errors.ErrGateShutdown
	} else {
		return p.gate.session.unsubscribe(kind, targets, channel)
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
