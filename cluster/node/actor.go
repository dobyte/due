package node

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/petermattis/goid"
)

// Creator is the function that creates an actor processor.
type Creator func(actor *Actor, args ...any) Processor

// The states of an [Actor].
const (
	unstart   int32 = iota // unstart: the actor has not been started
	started                // started: the actor is running
	destroyed              // destroyed: the actor has been destroyed
)

// Actor is an actor model.
//
// An actor owns an independent message queue and task queue, which keeps its internal processing
// thread-safe.
type Actor struct {
	opts                *actorOptions          // Actor options
	pid                 string                 // Unique identifier (Kind/ID), cached at creation time to avoid repeated concatenation
	scheduler           *Scheduler             // Scheduler
	state               atomic.Int32           // Actor state
	routes              map[int32]RouteHandler // Route handlers
	events              sync.Map               // Event handlers
	defaultRouteHandler RouteHandler           // Default route handler
	processor           Processor              // Processor
	rw                  *sync.RWMutex          // Read-write lock
	taskQueue           *queue.Tasker          // Task queue
	messageQueue        *queue.Queue[Context]  // Message queue
	binds               sync.Map               // Bound users
	registered          atomic.Bool            // Whether the actor has been registered in the scheduler's kind reference count
	dispatchGoid        atomic.Int64           // Goroutine ID of the dispatcher
}

// ID returns the actor ID.
func (a *Actor) ID() string {
	return a.opts.id
}

// PID returns the unique identifier of the actor.
//
// The identifier is composed of the kind and the ID and is used to globally locate the actor.
func (a *Actor) PID() string {
	return a.pid
}

// Kind returns the actor kind.
func (a *Actor) Kind() string {
	return a.opts.kind
}

// Spawn derives a new actor from the actor.
//
// creator creates the processor of the new actor and opts configures it. It reports an error when
// an actor with the same kind and ID already exists or when creation fails.
func (a *Actor) Spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	return a.scheduler.spawn(creator, opts...)
}

// Proxy returns the node proxy.
func (a *Actor) Proxy() *Proxy {
	return a.scheduler.node.proxy
}

// Invoke calls f in a thread-safe manner inside the actor.
//
// The call is written to the actor's task queue and executed serially; in blocking mode (wait is
// true) it waits for the function to complete and by default it does not wait. A synchronous call
// made from the actor's own dispatcher goroutine is executed directly, avoiding a deadlock caused
// by waiting on the queue the caller is running in. Do not wait synchronously across queues in the
// dispatch chain, for example an actor task that waits synchronously for a node task while that
// node task waits synchronously for an actor task; the mutual wait forms a cross-queue circular
// wait deadlock.
func (a *Actor) Invoke(f func(), wait ...bool) error {
	if !a.started() {
		return errors.ErrActorNotStarted
	}

	if len(wait) > 0 && wait[0] && a.dispatchGoid.Load() == goid.Get() {
		xcall.Call(f)
	} else {
		a.rw.RLock()
		wg, err := a.taskQueue.Commit(f, wait...)
		a.rw.RUnlock()

		if err != nil {
			return err
		}

		if wg != nil {
			wg.Wait()
		}
	}

	return nil
}

// AfterFunc schedules f to run after d, matching the semantics of [time.AfterFunc]. The returned
// [Timer] can be cancelled with Timer.Stop.
func (a *Actor) AfterFunc(d time.Duration, f func()) (*Timer, error) {
	if !a.started() {
		return nil, errors.ErrActorNotStarted
	}

	timer := time.AfterFunc(d, func() {
		if a.started() {
			xcall.Call(f)
		} else {
			log.Warnf("actor %s exec task failed, err: %v", a.PID(), errors.ErrActorNotStarted)
		}
	})

	return &Timer{timer: timer}, nil
}

// AfterInvoke schedules f to run after d in a thread-safe manner.
//
// After the delay, f is executed serially through the task queue, which keeps the actor
// thread-safe. The returned [Timer] can be cancelled with Timer.Stop.
func (a *Actor) AfterInvoke(d time.Duration, f func()) (*Timer, error) {
	if !a.started() {
		return nil, errors.ErrActorNotStarted
	}

	timer := time.AfterFunc(d, func() {
		var err error

		a.rw.RLock()
		if a.started() {
			_, err = a.taskQueue.Commit(f)
		} else {
			err = errors.ErrActorNotStarted
		}
		a.rw.RUnlock()

		if err != nil {
			log.Warnf("actor %s exec task failed, err: %v", a.PID(), err)
		}
	})

	return &Timer{timer: timer}, nil
}

// SetDefaultRouteHandler sets the default route handler, which handles every route that has not
// been registered.
func (a *Actor) SetDefaultRouteHandler(handler RouteHandler) {
	a.rw.RLock()
	defer a.rw.RUnlock()

	switch a.state.Load() {
	case unstart:
		a.defaultRouteHandler = handler
	case started:
		if _, err := a.taskQueue.Commit(func() {
			if a.started() {
				a.defaultRouteHandler = handler
			}
		}); err != nil {
			log.Warnf("set default route handler failed, err: %v", err)
		}
	}
}

// AddRouteHandler registers handler for the given route.
func (a *Actor) AddRouteHandler(route int32, handler RouteHandler) {
	a.rw.RLock()
	defer a.rw.RUnlock()

	switch a.state.Load() {
	case unstart:
		a.routes[route] = handler
	case started:
		if _, err := a.taskQueue.Commit(func() {
			if a.started() {
				a.routes[route] = handler

				if a.opts.dispatch {
					a.scheduler.routes.Store(route, a.Kind())

					if val, ok := a.scheduler.kinds.Load(a.Kind()); ok {
						val.(*kindEntity).routes.Store(route, struct{}{})
					}
				}
			}
		}); err != nil {
			log.Warnf("add route handler %d failed, err: %v", route, err)
		}
	}
}

// AddEventHandler registers handler for the given event.
func (a *Actor) AddEventHandler(event cluster.Event, handler EventHandler) {
	a.rw.RLock()
	defer a.rw.RUnlock()

	switch a.state.Load() {
	case unstart:
		a.events.Store(event, handler)
	case started:
		if _, err := a.taskQueue.Commit(func() {
			if a.started() {
				a.events.Store(event, handler)
			}
		}); err != nil {
			log.Warnf("add event handler %s failed, err: %v", event, err)
		}
	}
}

// Next delivers ctx to the actor for processing.
//
// The context is written to the actor's message queue and processed serially. It reports an error
// when the actor has not been started or when enqueuing the message fails.
func (a *Actor) Next(ctx Context) error {
	a.rw.RLock()
	defer a.rw.RUnlock()

	if a.state.Load() != started {
		return errors.ErrActorNotStarted
	}

	ctx.storeActor(a)
	ctx.incrVersion()
	ctx.cancelDefer()

	if err := a.messageQueue.Write(ctx); err != nil {
		ctx.deleteActor()
		ctx.decrVersion()
		ctx.recoverDefer()
		return err
	}

	return nil
}

// Deliver delivers message to the current actor for the given user.
//
// It reports an error when packing the message or delivering it fails.
func (a *Actor) Deliver(uid int64, message *cluster.Message) error {
	buf, err := a.scheduler.node.proxy.PackBuffer(message.Data)
	if err != nil {
		return err
	}

	req := a.scheduler.node.reqPool.Get().(*request)
	req.nid = a.scheduler.node.opts.id
	req.pid = a.PID()
	req.uid = uid
	req.seq = message.Seq
	req.route = message.Route
	req.message = buf

	if a.scheduler.node.opts.ctxFunc != nil {
		req.ctx = a.scheduler.node.opts.ctxFunc()
	} else {
		req.ctx = context.Background()
	}

	if err := a.Next(req); err != nil {
		req.release()
		return err
	}

	return nil
}

// Push pushes message to the local node queue for the given user.
//
// It reports an error when packing the message fails.
func (a *Actor) Push(uid int64, message *cluster.Message) error {
	buf, err := a.scheduler.node.proxy.PackBuffer(message.Data)
	if err != nil {
		return err
	}

	return a.scheduler.node.router.deliver("", a.scheduler.node.opts.id, a.PID(), 0, uid, message.Seq, message.Route, buf)
}

// Destroy destroys the actor. It reports false when the actor does not exist or has already been
// destroyed.
func (a *Actor) Destroy() (ok bool) {
	if ok = a.destroy(); !ok {
		return
	}

	_, ok = a.scheduler.remove(a.Kind(), a.ID())
	return
}

// destroy destroys the actor.
//
// It unbinds all users, closes the task and message queues, releases the residual messages in the
// queues and invokes the processor's Destroy method. It reports false when the actor is not in the
// started state.
func (a *Actor) destroy() bool {
	if !a.state.CompareAndSwap(started, destroyed) {
		return false
	}

	if a.opts.dispatch && a.registered.Load() {
		a.scheduler.releaseKind(a.Kind())
	}

	a.scheduler.unbindAllActor(a)

	a.rw.Lock()
	processor := a.processor
	a.processor = nil
	a.taskQueue.Close()
	a.messageQueue.Close()
	a.rw.Unlock()

	// Release all tasks left in the task queue.
	a.taskQueue.Clean()

	// Release all messages left in the message queue.
	a.messageQueue.Clean(func(ctx Context) { ctx.release() })

	if processor != nil {
		xcall.Call(processor.Destroy)
	}

	if a.opts.wait {
		a.scheduler.node.doDoneWait()
	}

	return true
}

// bindUser binds the user with the given uid to the actor.
func (a *Actor) bindUser(uid int64) {
	a.binds.Store(uid, struct{}{})
}

// unbindUser unbinds the user with the given uid. It reports whether the user was bound to the
// actor.
func (a *Actor) unbindUser(uid int64) bool {
	_, ok := a.binds.LoadAndDelete(uid)
	return ok
}

// dispatch dispatches messages and tasks.
//
// It loops over the message queue and the task queue, handling event or route messages and tasks
// respectively, and exits when the queues are closed.
func (a *Actor) dispatch() {
	a.dispatchGoid.Store(goid.Get())

	for {
		select {
		case ctx, ok := <-a.messageQueue.Read():
			if !ok {
				return
			}

			version := ctx.loadVersion()

			if a.started() {
				ctx.releaseDefer()

				if ctx.Kind() == Event {
					if v, ok := a.events.Load(ctx.Event()); ok {
						if handler, ok := v.(EventHandler); ok {
							xcall.Call(func() { handler(ctx) })

							ctx.compareVersionExecDefer(version)
						}
					}
				} else {
					if handler, ok := a.routes[ctx.Route()]; ok {
						xcall.Call(func() { handler(ctx) })

						ctx.compareVersionExecDefer(version)
					} else if a.defaultRouteHandler != nil {
						xcall.Call(func() { a.defaultRouteHandler(ctx) })

						ctx.compareVersionExecDefer(version)
					}
				}
			}

			ctx.compareVersionRecycle(version)
		case task, ok := <-a.taskQueue.Read():
			if !ok {
				return
			}

			a.taskQueue.Handle(task, a.started())
		}
	}
}

// started reports whether the actor is in the started state.
func (a *Actor) started() bool {
	return a.state.Load() == started
}
