package node

import (
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

// relationShardNum is the number of user binding relation shards (a power of two, so that the shard
// can be located with a bit operation).
const relationShardNum = 256

// relationShard is a shard of user binding relations.
//
// Relations are sharded by the low bits of the UID to dilute read-write lock contention; the struct
// is padded to a 64-byte cache line to eliminate false sharing between adjacent shards.
type relationShard struct {
	rw        sync.RWMutex
	relations map[int64]map[string]*Actor
	_         [32]byte
}

// Scheduler is a scheduler.
//
// It is responsible for creating and destroying actors as well as maintaining the relations between
// users and actors and dispatching messages.
type Scheduler struct {
	node   *Node
	rw     sync.RWMutex // Protects the lifetime consistency of actors/kinds/routes
	actors sync.Map
	routes sync.Map
	kinds  sync.Map
	shards [relationShardNum]relationShard // Shards of the binding relations between users and actors
}

// kindEntity is a kind entity.
//
// It maintains the reference count of the schedulable actors of a kind and the set of routes
// registered by that kind. When the last actor is released, the scheduling routes are cleaned up
// precisely by the route set, avoiding a full scan of the route table.
type kindEntity struct {
	count  atomic.Int32 // Reference count of the schedulable actors of the kind
	routes sync.Map     // Set of routes registered by the kind (route int32 → struct{})
}

// newScheduler creates a new scheduler for the given node server.
func newScheduler(node *Node) *Scheduler {
	s := &Scheduler{node: node}
	for i := range s.shards {
		s.shards[i].relations = make(map[int64]map[string]*Actor)
	}
	return s
}

// shard locates the binding relation shard for the given user ID.
func (s *Scheduler) shard(uid int64) *relationShard {
	return &s.shards[uint64(uid)&(relationShardNum-1)]
}

// spawn creates a new actor.
//
// It creates the actor, initializes its processor, registers it to the scheduler and then starts its
// message dispatch. It returns an error when the actor already exists or fails to be created.
func (s *Scheduler) spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	o := defaultActorOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.kind == "" || o.id == "" {
		return nil, errors.ErrInvalidArgument
	}

	if _, ok := s.load(o.kind, o.id); ok {
		return nil, errors.ErrActorExists
	}

	if o.wait && !s.node.doAddWait() {
		return nil, errors.ErrNodeShutdown
	}

	act := &Actor{}
	act.opts = o
	act.pid = o.kind + "/" + o.id
	act.scheduler = s
	act.state.Store(started)
	act.routes = make(map[int32]RouteHandler)
	act.rw = &sync.RWMutex{}
	act.taskQueue = queue.NewTasker(o.taskQueueSize, o.taskWriteTimeout)
	act.messageQueue = queue.NewQueue[Context](o.messageQueueSize, o.messageWriteTimeout)

	xcall.Call(func() {
		if act.processor = creator(act, o.args...); act.processor != nil {
			act.processor.Init()
		}
	})

	// The actor processor failed to be created.
	if act.processor == nil {
		act.destroy()
		return nil, errors.ErrActorCreateFailed
	}

	s.rw.Lock()

	if _, ok := s.load(o.kind, o.id); ok {
		s.rw.Unlock()
		act.destroy()
		return nil, errors.ErrActorExists
	}

	if act.opts.dispatch {
		val, ok := s.kinds.Load(act.Kind())
		if !ok {
			entity := &kindEntity{}
			for route := range act.routes {
				entity.routes.Store(route, struct{}{})
				s.routes.Store(route, act.Kind())
			}
			s.kinds.Store(act.Kind(), entity)
			val = entity
		}
		val.(*kindEntity).count.Add(1)
		act.registered.Store(true)
	}

	s.actors.Store(act.PID(), act)
	s.rw.Unlock()

	xcall.Go(act.dispatch)
	xcall.Call(act.processor.Start)

	return act, nil
}

// kill kills an actor. It reports whether the actor was killed successfully.
func (s *Scheduler) kill(kind, id string) bool {
	if act, ok := s.remove(kind, id); ok {
		return act.destroy()
	} else {
		return false
	}
}

// remove removes an actor from the scheduler's actor table. The user binding relations are cleaned
// up by destroy. It returns the removed actor instance and whether the actor exists.
func (s *Scheduler) remove(kind, id string) (*Actor, bool) {
	s.rw.Lock()
	defer s.rw.Unlock()

	act, ok := s.load(kind, id)
	if !ok {
		return nil, false
	}

	s.actors.Delete(act.PID())

	return act, true
}

// load loads an actor. It returns the actor instance and whether the actor exists.
func (s *Scheduler) load(kind, id string) (*Actor, bool) {
	return s.doLoad(kind + "/" + id)
}

// doLoad loads an actor by its unique PID (kind/id). It returns the actor instance and whether the
// actor exists.
func (s *Scheduler) doLoad(pid string) (*Actor, bool) {
	if actor, ok := s.actors.Load(pid); ok {
		return actor.(*Actor), true
	}

	return nil, false
}

// bindActor establishes a binding relation between a user and an actor.
//
// The relation is written into an independent shard hashed by the UID, so that the global lock does
// not block the message dispatch of other users. It returns an error when the user ID is illegal or
// the actor does not exist or has been destroyed.
func (s *Scheduler) bindActor(uid int64, kind, id string) error {
	if uid == 0 {
		return errors.ErrIllegalOperation
	}

	act, ok := s.load(kind, id)
	if !ok {
		return errors.ErrNotFoundActor
	}

	// Verify that the actor is still started, so that a concurrent kill/destroy does not leave a
	// dangling relation to a destroyed actor.
	if !act.started() {
		return errors.ErrActorNotStarted
	}

	// Register into the actor's binding table before writing the scheduler relation, so that a
	// concurrent destroy scanning binds can clean up this binding.
	act.bindUser(uid)

	sh := s.shard(uid)

	sh.rw.Lock()
	relations, ok := sh.relations[uid]
	if !ok {
		relations = make(map[string]*Actor)
		sh.relations[uid] = relations
	}
	relations[act.Kind()] = act
	sh.rw.Unlock()

	// Re-verify the actor state: when racing with kill/destroy, reclaim the binding that has been
	// written to avoid a dangling relation.
	if !act.started() {
		act.unbindUser(uid)
		s.doUnbindActor(uid, act.Kind(), act)
		return errors.ErrActorNotStarted
	}

	return nil
}

// unbindActor removes the binding relation between a user and an actor. It returns an error when the
// user or actor relation does not exist.
func (s *Scheduler) unbindActor(uid int64, kind string) error {
	sh := s.shard(uid)

	sh.rw.RLock()
	relations, ok := sh.relations[uid]
	if !ok {
		sh.rw.RUnlock()
		return errors.ErrNotFoundActor
	}
	act, ok := relations[kind]
	sh.rw.RUnlock()

	if !ok {
		return errors.ErrNotFoundActor
	}

	if act.unbindUser(uid) {
		s.doUnbindActor(uid, kind, act)
	}

	return nil
}

// unbindAllActor removes all user binding relations of an actor.
//
// Each shard is located and locked per user, so that scanning the binding table does not hold the
// global lock and block the message dispatch of the whole node.
func (s *Scheduler) unbindAllActor(act *Actor) {
	act.binds.Range(func(k, _ any) bool {
		s.doUnbindActor(k.(int64), act.Kind(), act)

		act.binds.Delete(k)

		return true
	})
}

// doUnbindActor removes the binding relation between a user and an actor.
//
// It deletes the relation only when the current binding still points to act, preventing the removal
// of a relation the user has re-bound to a new actor of the same kind. It reports whether the
// unbinding succeeded.
func (s *Scheduler) doUnbindActor(uid int64, kind string, act *Actor) bool {
	sh := s.shard(uid)

	sh.rw.Lock()
	relations, ok := sh.relations[uid]
	if !ok {
		sh.rw.Unlock()
		return false
	}

	current, ok := relations[kind]
	if !ok || current != act {
		sh.rw.Unlock()
		return false
	}

	delete(relations, kind)

	if len(relations) == 0 {
		delete(sh.relations, uid)
	}
	sh.rw.Unlock()

	return true
}

// releaseKind releases a kind reference.
//
// When the last schedulable actor of a kind is destroyed, the kind and its route mapping are cleaned
// up to avoid accumulating stale entries.
func (s *Scheduler) releaseKind(kind string) {
	s.rw.Lock()
	defer s.rw.Unlock()

	val, ok := s.kinds.Load(kind)
	if !ok {
		return
	}

	entity := val.(*kindEntity)

	if entity.count.Add(-1) > 0 {
		return
	}

	s.kinds.Delete(kind)

	// Only clean up routes that still point to the current kind, avoiding the removal of route
	// mappings overwritten by another kind.
	entity.routes.Range(func(route, _ any) bool {
		s.routes.CompareAndDelete(route, kind)

		return true
	})
}

// loadActor returns the actor bound to a user.
//
// It is called for every dispatched message and locks the shard by UID read-only, avoiding cache
// line contention caused by the global lock under high-concurrency dispatch.
func (s *Scheduler) loadActor(uid int64, kind string) (*Actor, bool) {
	sh := s.shard(uid)

	sh.rw.RLock()
	relations, ok := sh.relations[uid]
	if ok {
		if act, ok := relations[kind]; ok {
			sh.rw.RUnlock()
			return act, true
		}
	}
	sh.rw.RUnlock()

	return nil, false
}

// dispatch dispatches a message. It dispatches to the request or event handling flow according to
// the context kind.
func (s *Scheduler) dispatch(ctx Context) error {
	if ctx.Kind() == Request {
		return s.dispatchRequest(ctx)
	} else {
		return s.dispatchEvent(ctx)
	}
}

// dispatchRequest dispatches a request. It locates the actor bound to the user by the route number
// and delivers the message. It returns an error when the user ID is illegal, the route is not
// registered or the user is not bound to an actor.
func (s *Scheduler) dispatchRequest(ctx Context) error {
	uid := ctx.UID()

	if uid == 0 {
		return errors.ErrMissingDispatchStrategy
	}

	kind, ok := s.routes.Load(ctx.Route())
	if !ok {
		return errors.ErrUnregisterRoute
	}

	act, ok := s.loadActor(uid, kind.(string))
	if !ok {
		log.Errorf("dispatch request failed, uid = %v route = %v kind = %v", uid, ctx.Route(), kind)
		return errors.ErrNotBindActor
	}

	return act.Next(ctx)
}

// dispatchEvent dispatches an event. It clones the context and delivers it to every schedulable
// actor.
func (s *Scheduler) dispatchEvent(ctx Context) error {
	s.actors.Range(func(_, actor any) bool {
		if act, ok := actor.(*Actor); ok && act.opts.dispatch {
			if _, ok = act.events.Load(ctx.Event()); ok {
				c := ctx.Clone()

				if err := act.Next(c); err != nil {
					c.release()
				}
			}
		}

		return true
	})

	return nil
}
