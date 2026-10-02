package node

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/chains"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/session"
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/transport"
)

// event is an event context.
type event struct {
	node    *Node                 // Node server
	ctx     context.Context       // Context
	gid     string                // Gateway ID
	cid     int64                 // Connection ID
	uid     int64                 // User ID
	event   cluster.Event         // Event type
	version atomic.Int32          // Object version
	chain   *chains.Chain         // Defer call chain
	actor   atomic.Pointer[Actor] // Current actor
}

// GID returns the gateway ID.
func (e *event) GID() string {
	return e.gid
}

// NID returns the node ID.
func (e *event) NID() string {
	return ""
}

// CID returns the connection ID.
func (e *event) CID() int64 {
	return e.cid
}

// UID returns the user ID.
func (e *event) UID() int64 {
	return e.uid
}

// Seq returns the message sequence number.
func (e *event) Seq() int32 {
	return 0
}

// Route returns the message route number.
func (e *event) Route() int32 {
	return 0
}

// Event returns the event type.
func (e *event) Event() cluster.Event {
	return e.event
}

// Kind returns the message kind.
func (e *event) Kind() Kind {
	return Event
}

// Parse parses the message.
func (e *event) Parse(v any) error {
	return errors.NewError(errors.ErrIllegalOperation)
}

// Defer adds a deferred call to the call stack.
//
// This method behaves like the go defer statement and its scope is limited to the current handler
// function, so it is recommended over go defer. The difference is that Defer allows the call stack
// to be cancelled. The stack is cancelled automatically when Task or Next is called, and it can
// also be cancelled manually with Cancel. The optional bottom controls whether fn is pushed to the
// bottom of the stack.
func (e *event) Defer(fn func(), bottom ...bool) {
	if e.chain == nil {
		e.chain = chains.NewChain()
	}

	if len(bottom) > 0 && bottom[0] {
		e.chain.AddToTail(fn)
	} else {
		e.chain.AddToHead(fn)
	}
}

// Cancel cancels the defer call stack.
func (e *event) Cancel() {
	e.releaseDefer()
}

// compareVersionExecDefer executes the defer call stack when the version matches.
func (e *event) compareVersionExecDefer(version int32) {
	if e.chain != nil && e.version.Load() == version {
		e.chain.FireHead()
	}
}

// Clone clones the context.
func (e *event) Clone() Context {
	c := e.node.evtPool.Get().(*event)
	c.gid = e.gid
	c.cid = e.cid
	c.uid = e.uid
	c.ctx = e.ctx
	c.event = e.event
	c.actor.Store(e.actor.Load())

	return c
}

// Task submits a task.
//
// It automatically cancels every function in the defer call stack.
func (e *event) Task(fn func(ctx Context)) {
	if !e.node.doAddWait() {
		return
	}

	version := e.incrVersion()

	e.recoverDefer()

	taskpool.Add(func() {
		defer func() {
			e.compareVersionExecDefer(version)

			e.compareVersionRecycle(version)

			e.node.doDoneWait()
		}()

		fn(e)
	})
}

// Next dispatches the message down.
//
// It automatically cancels every function in the defer call stack.
func (e *event) Next() error {
	return e.node.scheduler.dispatch(e)
}

// Proxy returns the node proxy.
func (e *event) Proxy() *Proxy {
	return e.node.proxy
}

// Context returns the underlying standard context.
func (e *event) Context() context.Context {
	return e.ctx
}

// SetValue sets a value on the context.
func (e *event) SetValue(key, val any) {
	e.ctx = context.WithValue(e.ctx, key, val)
}

// GetValue returns the value associated with key.
func (e *event) GetValue(key any) any {
	return e.ctx.Value(key)
}

// BindGate binds the gateway.
func (e *event) BindGate(uid ...int64) error {
	switch {
	case len(uid) > 0:
		if err := e.node.proxy.BindGate(e.ctx, e.gid, e.cid, uid[0]); err != nil {
			return err
		}

		e.uid = uid[0]

		return nil
	case e.uid != 0:
		return e.node.proxy.BindGate(e.ctx, e.gid, e.cid, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// UnbindGate unbinds the gateway.
func (e *event) UnbindGate(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.node.proxy.UnbindGate(e.ctx, uid[0])
	case e.uid != 0:
		return e.node.proxy.UnbindGate(e.ctx, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// BindNode binds the node.
func (e *event) BindNode(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.node.proxy.BindNode(e.ctx, uid[0])
	case e.uid != 0:
		return e.node.proxy.BindNode(e.ctx, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// UnbindNode unbinds the node.
func (e *event) UnbindNode(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return e.node.proxy.UnbindNode(e.ctx, uid[0])
	case e.uid != 0:
		return e.node.proxy.UnbindNode(e.ctx, e.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// Subscribe subscribes to a channel.
func (e *event) Subscribe(channel string, uids ...int64) error {
	if len(uids) > 0 {
		return e.node.proxy.Subscribe(e.ctx, &cluster.SubscribeArgs{
			Kind:    session.User,
			Targets: uids,
			Channel: channel,
		})
	} else {
		if e.gid == "" {
			return errors.ErrIllegalOperation
		}

		return e.node.proxy.Subscribe(e.ctx, &cluster.SubscribeArgs{
			GID:     e.gid,
			Kind:    session.Conn,
			Targets: []int64{e.cid},
			Channel: channel,
		})
	}
}

// Unsubscribe unsubscribes from a channel.
func (e *event) Unsubscribe(channel string, uids ...int64) error {
	if len(uids) > 0 {
		return e.node.proxy.Unsubscribe(e.ctx, &cluster.UnsubscribeArgs{
			Kind:    session.User,
			Targets: uids,
			Channel: channel,
		})
	} else {
		if e.gid == "" {
			return errors.ErrIllegalOperation
		}

		return e.node.proxy.Unsubscribe(e.ctx, &cluster.UnsubscribeArgs{
			GID:     e.gid,
			Kind:    session.Conn,
			Targets: []int64{e.cid},
			Channel: channel,
		})
	}
}

// BindActor binds an actor.
func (e *event) BindActor(kind, id string) error {
	return e.node.scheduler.bindActor(e.uid, kind, id)
}

// UnbindActor unbinds the actor.
func (e *event) UnbindActor(kind string) {
	e.node.scheduler.unbindActor(e.uid, kind)
}

// Spawn derives a new actor.
func (e *event) Spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	return e.node.scheduler.spawn(creator, opts...)
}

// Kill kills an existing actor.
func (e *event) Kill(kind, id string) bool {
	return e.node.scheduler.kill(kind, id)
}

// Actor returns the actor with the given kind and ID.
func (e *event) Actor(kind, id string) (*Actor, bool) {
	return e.node.scheduler.load(kind, id)
}

// Invoke calls a function in a thread-safe manner.
//
// Inside a global handler, it calls proxy.Invoke; inside an actor processor, it calls
// actor.Invoke. isBlock controls whether the call is blocking; it is blocking by default.
func (e *event) Invoke(fn func(), isBlock ...bool) error {
	if actor := e.actor.Load(); actor != nil {
		return actor.Invoke(fn, isBlock...)
	} else {
		return e.node.proxy.Invoke(fn, isBlock...)
	}
}

// AfterFunc schedules f to run after d, matching the semantics of [time.AfterFunc].
//
// Inside a global handler, it calls proxy.AfterFunc; inside an actor processor, it calls
// actor.AfterFunc.
func (e *event) AfterFunc(d time.Duration, f func()) (*Timer, error) {
	if actor := e.actor.Load(); actor != nil {
		return actor.AfterFunc(d, f)
	} else {
		return e.node.proxy.AfterFunc(d, f)
	}
}

// AfterInvoke schedules f to run after d in a thread-safe manner.
//
// Inside a global handler, it calls proxy.AfterInvoke; inside an actor processor, it calls
// actor.AfterInvoke.
func (e *event) AfterInvoke(d time.Duration, f func()) (*Timer, error) {
	if actor := e.actor.Load(); actor != nil {
		return actor.AfterInvoke(d, f)
	} else {
		return e.node.proxy.AfterInvoke(d, f)
	}
}

// GetIP returns the client IP.
func (e *event) GetIP() (string, error) {
	if e.gid == "" {
		return "", errors.ErrIllegalOperation
	}

	return e.node.proxy.GetIP(e.ctx, &cluster.GetIPArgs{
		GID:    e.gid,
		Kind:   session.Conn,
		Target: e.cid,
	})
}

// Deliver delivers a message to a node for processing.
func (e *event) Deliver(args *cluster.DeliverArgs) error {
	return e.node.proxy.Deliver(e.ctx, args)
}

// Reply replies with a message.
func (e *event) Reply(message *cluster.Message) error {
	if e.uid != 0 {
		return e.node.proxy.Push(e.ctx, &cluster.PushArgs{
			GID:     e.gid,
			Kind:    session.User,
			Target:  e.uid,
			Message: message,
		})
	} else {
		return e.node.proxy.Push(e.ctx, &cluster.PushArgs{
			GID:     e.gid,
			Kind:    session.Conn,
			Target:  e.cid,
			Message: message,
		})
	}
}

// Response responds to the current route message.
func (e *event) Response(message any) error {
	return errors.NewError(errors.ErrIllegalOperation)
}

// Disconnect closes the connection from the gateway.
func (e *event) Disconnect(force ...bool) error {
	return e.node.proxy.Disconnect(e.ctx, &cluster.DisconnectArgs{
		GID:    e.gid,
		Kind:   session.Conn,
		Target: e.cid,
		Force:  len(force) > 0 && force[0],
	})
}

// NewMeshClient creates a new mesh service client.
//
// The target can be given in three forms:
//   - direct connection: direct://127.0.0.1:8011
//   - direct connection: direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
//   - service discovery: discovery://service_name
func (e *event) NewMeshClient(target string) (transport.Client, error) {
	return e.node.proxy.NewMeshClient(target)
}

// storeActor stores the current actor.
func (e *event) storeActor(actor *Actor) {
	e.actor.Store(actor)
}

// deleteActor deletes the current actor.
func (e *event) deleteActor() {
	e.actor.Store(nil)
}

// incrVersion increments the version.
func (e *event) incrVersion() int32 {
	return e.version.Add(1)
}

// decrVersion decrements the version.
func (e *event) decrVersion() int32 {
	return e.version.Add(-1)
}

// loadVersion returns the current version.
func (e *event) loadVersion() int32 {
	return e.version.Load()
}

// cancelDefer cancels the defer call stack.
func (e *event) cancelDefer() {
	if e.chain != nil {
		e.chain.Cancel()
	}
}

// recoverDefer recovers the defer call stack.
func (e *event) recoverDefer() {
	if e.chain != nil {
		e.chain.Recover()
	}
}

// releaseDefer releases the defer call stack.
func (e *event) releaseDefer() {
	if e.chain != nil {
		e.chain.Release()
	}
}

// compareVersionRecycle recycles the object when the version matches.
func (e *event) compareVersionRecycle(version int32) {
	if e.version.CompareAndSwap(version, 0) {
		e.release()
	}
}

// release releases the event object.
func (e *event) release() {
	e.ctx = context.Background()
	e.gid = ""
	e.cid = 0
	e.uid = 0
	e.event = 0
	e.version.Store(0)
	e.actor.Store(nil)

	if e.chain != nil {
		e.chain.Release()
		e.chain = nil
	}

	e.node.evtPool.Put(e)
}
