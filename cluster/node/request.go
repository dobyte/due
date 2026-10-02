// Author: fuxiao
// Email: 576101059@qq.com
// Date: 2022/6/19 12:20 PM
// Desc: TODO

package node

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/chains"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/session"
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/jinzhu/copier"
)

// request is a request context.
type request struct {
	node    *Node
	ctx     context.Context       // Context
	gid     string                // Source gateway ID
	nid     string                // Source node ID
	pid     string                // Source actor ID
	cid     int64                 // Connection ID
	uid     int64                 // User ID
	seq     int32                 // Message sequence number
	route   int32                 // Message route number
	message any                   // Message data
	cache   []byte                // Serialized message cache, reused across clones to avoid re-serializing
	version atomic.Int32          // Version number
	chain   *chains.Chain         // Call chain
	actor   atomic.Pointer[Actor] // Current actor
}

// GID returns the gateway ID.
func (r *request) GID() string {
	return r.gid
}

// NID returns the node ID.
func (r *request) NID() string {
	return r.nid
}

// CID returns the connection ID.
func (r *request) CID() int64 {
	return r.cid
}

// UID returns the user ID.
func (r *request) UID() int64 {
	return r.uid
}

// Seq returns the message sequence number.
func (r *request) Seq() int32 {
	return r.seq
}

// Route returns the message route number.
func (r *request) Route() int32 {
	return r.route
}

// Event returns the event type, which is always 0 for a request.
func (r *request) Event() cluster.Event {
	return 0
}

// Kind returns the message kind of the context.
func (r *request) Kind() Kind {
	return Request
}

// Parse parses the message into v.
func (r *request) Parse(v any) error {
	var msg []byte

	switch m := r.message.(type) {
	case buffer.Buffer:
		msg = m.Bytes()
	case []byte:
		msg = m
	default:
		return copier.CopyWithOption(v, m, copier.Option{
			DeepCopy: true,
		})
	}

	if len(msg) == 0 {
		return nil
	}

	if r.gid != "" && r.node.opts.encryptor != nil {
		data, err := r.node.opts.encryptor.Decrypt(msg)
		if err != nil {
			return err
		}

		return r.node.opts.codec.Unmarshal(data, v)
	}

	return r.node.opts.codec.Unmarshal(msg, v)
}

// Defer adds a deferred call to the defer call chain.
//
// It behaves like the go defer statement but is scoped to the current handler function only, and it
// is recommended over go defer. The difference is that the chain can be cancelled: calling Task or
// Next cancels the chain automatically, and Cancel cancels it manually. bottom reports whether fn is
// attached to the bottom of the chain instead of the top.
func (r *request) Defer(fn func(), bottom ...bool) {
	if r.chain == nil {
		r.chain = chains.NewChain()
	}

	if len(bottom) > 0 && bottom[0] {
		r.chain.AddToTail(fn)
	} else {
		r.chain.AddToHead(fn)
	}
}

// Cancel cancels the defer call chain.
func (r *request) Cancel() {
	if r.chain != nil {
		r.chain.Release()
	}
}

// compareVersionExecDefer fires the head of the defer call chain when the given version is still
// current.
func (r *request) compareVersionExecDefer(version int32) {
	if r.chain != nil && r.loadVersion() == version {
		r.chain.FireHead()
	}
}

// Clone clones the Context.
func (r *request) Clone() Context {
	c := r.node.reqPool.Get().(*request)
	c.ctx = r.ctx
	c.gid = r.gid
	c.nid = r.nid
	c.cid = r.cid
	c.uid = r.uid
	c.pid = r.pid
	c.seq = r.seq
	c.route = r.route
	c.actor.Store(r.actor.Load())

	switch m := r.message.(type) {
	case buffer.Buffer:
		message := make([]byte, m.Len())
		offset := 0
		m.VisitBytes(func(bytes []byte) bool {
			offset += copy(message[offset:], bytes)
			return true
		})
		c.message = message
	case []byte:
		message := make([]byte, len(m))
		copy(message, m)
		c.message = message
	default:
		// Cache the serialization result on the request so that repeated clones (for example when
		// forwarding a message to multiple actors) serialize only once.
		if r.cache == nil {
			if msg, err := json.Marshal(m); err != nil {
				log.Warnf("marshal request message failed: %v", err)
			} else {
				r.cache = msg
			}
		}

		if r.cache != nil {
			message := make([]byte, len(r.cache))
			copy(message, r.cache)
			c.message = message
		}
	}

	return c
}

// Task submits a task.
//
// It is recommended over taskpool.Add and go func. Calling it cancels every function of the defer
// call chain automatically.
func (r *request) Task(fn func(ctx Context)) {
	if !r.node.doAddWait() {
		return
	}

	version := r.incrVersion()

	r.recoverDefer()

	taskpool.Add(func() {
		defer func() {
			r.compareVersionExecDefer(version)

			r.compareVersionRecycle(version)

			r.node.doDoneWait()
		}()

		fn(r)
	})
}

// Next dispatches the message to the next stage.
//
// Calling it cancels every function of the defer call chain automatically.
func (r *request) Next() error {
	return r.node.scheduler.dispatch(r)
}

// Proxy returns the proxy API.
func (r *request) Proxy() *Proxy {
	return r.node.proxy
}

// Context returns the context.
func (r *request) Context() context.Context {
	return r.ctx
}

// SetValue sets a value on the context.
func (r *request) SetValue(key, val any) {
	r.ctx = context.WithValue(r.ctx, key, val)
}

// GetValue returns the value associated with key from the context.
func (r *request) GetValue(key any) any {
	return r.ctx.Value(key)
}

// BindGate binds the gateway.
func (r *request) BindGate(uid ...int64) error {
	switch {
	case len(uid) > 0:
		if err := r.node.proxy.BindGate(r.ctx, r.gid, r.cid, uid[0]); err != nil {
			return err
		}

		r.uid = uid[0]

		return nil
	case r.uid != 0:
		return r.node.proxy.BindGate(r.ctx, r.gid, r.cid, r.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// UnbindGate unbinds the gateway.
func (r *request) UnbindGate(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return r.node.proxy.UnbindGate(r.ctx, uid[0])
	case r.uid != 0:
		return r.node.proxy.UnbindGate(r.ctx, r.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// BindNode binds the node.
func (r *request) BindNode(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return r.node.proxy.BindNode(r.ctx, uid[0])
	case r.uid != 0:
		return r.node.proxy.BindNode(r.ctx, r.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// UnbindNode unbinds the node.
func (r *request) UnbindNode(uid ...int64) error {
	switch {
	case len(uid) > 0:
		return r.node.proxy.UnbindNode(r.ctx, uid[0])
	case r.uid != 0:
		return r.node.proxy.UnbindNode(r.ctx, r.uid)
	default:
		return errors.ErrIllegalOperation
	}
}

// Subscribe subscribes to a channel.
func (r *request) Subscribe(channel string, uids ...int64) error {
	if len(uids) > 0 {
		return r.node.proxy.Subscribe(r.ctx, &cluster.SubscribeArgs{
			Kind:    session.User,
			Targets: uids,
			Channel: channel,
		})
	} else {
		if r.gid == "" {
			return errors.ErrIllegalOperation
		}

		return r.node.proxy.Subscribe(r.ctx, &cluster.SubscribeArgs{
			GID:     r.gid,
			Kind:    session.Conn,
			Targets: []int64{r.cid},
			Channel: channel,
		})
	}
}

// Unsubscribe unsubscribes from a channel.
func (r *request) Unsubscribe(channel string, uids ...int64) error {
	if len(uids) > 0 {
		return r.node.proxy.Unsubscribe(r.ctx, &cluster.UnsubscribeArgs{
			Kind:    session.User,
			Targets: uids,
			Channel: channel,
		})
	} else {
		if r.gid == "" {
			return errors.ErrIllegalOperation
		}

		return r.node.proxy.Unsubscribe(r.ctx, &cluster.UnsubscribeArgs{
			GID:     r.gid,
			Kind:    session.Conn,
			Targets: []int64{r.cid},
			Channel: channel,
		})
	}
}

// BindActor binds an actor.
func (r *request) BindActor(kind, id string) error {
	return r.node.scheduler.bindActor(r.uid, kind, id)
}

// UnbindActor unbinds an actor.
func (r *request) UnbindActor(kind string) {
	r.node.scheduler.unbindActor(r.uid, kind)
}

// Spawn creates a new actor.
func (r *request) Spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	return r.node.scheduler.spawn(creator, opts...)
}

// Kill kills an existing actor.
func (r *request) Kill(kind, id string) bool {
	return r.node.scheduler.kill(kind, id)
}

// Actor returns an actor.
func (r *request) Actor(kind, id string) (*Actor, bool) {
	return r.node.scheduler.load(kind, id)
}

// Invoke calls a function in a thread-safe way.
//
// In a global handler the context delegates to [Proxy.Invoke], and in an actor handler it delegates
// to [Actor.Invoke]. isBlock reports whether the call blocks, and it blocks by default.
func (r *request) Invoke(fn func(), isBlock ...bool) error {
	if actor := r.actor.Load(); actor != nil {
		return actor.Invoke(fn, isBlock...)
	} else {
		return r.node.proxy.Invoke(fn, isBlock...)
	}
}

// AfterFunc schedules a delayed call and is used the same way as [time.AfterFunc].
//
// In a global handler the context delegates to [Proxy.AfterFunc], and in an actor handler it
// delegates to [Actor.AfterFunc].
func (r *request) AfterFunc(d time.Duration, f func()) (*Timer, error) {
	if actor := r.actor.Load(); actor != nil {
		return actor.AfterFunc(d, f)
	} else {
		return r.node.proxy.AfterFunc(d, f)
	}
}

// AfterInvoke schedules a thread-safe delayed call.
//
// In a global handler the context delegates to [Proxy.AfterInvoke], and in an actor handler it
// delegates to [Actor.AfterInvoke].
func (r *request) AfterInvoke(d time.Duration, f func()) (*Timer, error) {
	if actor := r.actor.Load(); actor != nil {
		return actor.AfterInvoke(d, f)
	} else {
		return r.node.proxy.AfterInvoke(d, f)
	}
}

// GetIP returns the client IP.
func (r *request) GetIP() (string, error) {
	if r.gid == "" {
		return "", errors.ErrIllegalOperation
	}

	return r.node.proxy.GetIP(r.ctx, &cluster.GetIPArgs{
		GID:    r.gid,
		Kind:   session.Conn,
		Target: r.cid,
	})
}

// Deliver delivers a message to a node for handling.
func (r *request) Deliver(args *cluster.DeliverArgs) error {
	return r.node.proxy.Deliver(r.ctx, args)
}

// Reply replies with a message.
func (r *request) Reply(message *cluster.Message) error {
	switch {
	case r.gid != "": // From a gateway
		return r.node.proxy.Push(r.ctx, &cluster.PushArgs{
			GID:     r.gid,
			Kind:    session.Conn,
			Target:  r.cid,
			Message: message,
		})
	case r.pid != "": // From an actor
		if actor, ok := r.node.scheduler.doLoad(r.pid); ok {
			return actor.Deliver(r.uid, message)
		}

		return nil
	case r.nid != "": // From another node
		if r.nid == r.node.opts.id {
			return nil
		}

		return r.node.proxy.Deliver(r.ctx, &cluster.DeliverArgs{
			NID:     r.nid,
			UID:     r.uid,
			Message: message,
		})
	default:
		return errors.ErrIllegalOperation
	}
}

// Response responds with a message.
func (r *request) Response(message any) error {
	return r.Reply(&cluster.Message{
		Route: r.route,
		Seq:   r.seq,
		Data:  message,
	})
}

// Disconnect closes the connection that comes from a gateway.
func (r *request) Disconnect(force ...bool) error {
	if r.gid == "" {
		return errors.ErrIllegalOperation
	}

	return r.node.proxy.Disconnect(r.ctx, &cluster.DisconnectArgs{
		GID:    r.gid,
		Kind:   session.Conn,
		Target: r.cid,
		Force:  len(force) > 0 && force[0],
	})
}

// NewMeshClient creates a new microservice client.
//
// target supports three modes:
//
//	service direct mode:    direct://127.0.0.1:8011
//	service direct mode:    direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
//	service discovery mode: discovery://service_name
func (r *request) NewMeshClient(target string) (transport.Client, error) {
	return r.node.proxy.NewMeshClient(target)
}

// storeActor stores the current actor.
func (r *request) storeActor(actor *Actor) {
	r.actor.Store(actor)
}

// deleteActor clears the current actor.
func (r *request) deleteActor() {
	r.actor.Store(nil)
}

// incrVersion increments the version number.
func (r *request) incrVersion() int32 {
	return r.version.Add(1)
}

// decrVersion decrements the version number.
func (r *request) decrVersion() int32 {
	return r.version.Add(-1)
}

// loadVersion returns the version number.
func (r *request) loadVersion() int32 {
	return r.version.Load()
}

// cancelDefer cancels the defer call chain.
func (r *request) cancelDefer() {
	if r.chain != nil {
		r.chain.Cancel()
	}
}

// recoverDefer recovers the defer call chain.
func (r *request) recoverDefer() {
	if r.chain != nil {
		r.chain.Recover()
	}
}

// releaseDefer releases the defer call chain.
func (r *request) releaseDefer() {
	if r.chain != nil {
		r.chain.Release()
	}
}

// compareVersionRecycle recycles the request object after comparing the version number.
func (r *request) compareVersionRecycle(version int32) {
	if r.version.CompareAndSwap(version, 0) {
		if r.node.router.postRouteHandler != nil {
			xcall.Call(func() { r.node.router.postRouteHandler(r) })
		}

		r.release()
	}
}

// release releases the request object.
func (r *request) release() {
	if b, ok := r.message.(buffer.Buffer); ok {
		b.Release()
	}

	r.ctx = context.Background()
	r.gid = ""
	r.cid = 0
	r.uid = 0
	r.pid = ""
	r.nid = ""
	r.seq = 0
	r.route = 0
	r.message = nil
	r.cache = nil
	r.version.Store(0)
	r.actor.Store(nil)

	if r.chain != nil {
		r.chain.Release()
		r.chain = nil
	}

	r.node.reqPool.Put(r)
}
