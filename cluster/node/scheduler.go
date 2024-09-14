package node

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"sync"
)

type Scheduler struct {
	node      *Node
	mu        sync.Mutex
	actors    sync.Map
	routes    sync.Map
	kinds     sync.Map
	rw        sync.RWMutex
	relations map[int64]map[string]*Actor
}

func newScheduler(node *Node) *Scheduler {
	return &Scheduler{
		node:      node,
		relations: make(map[int64]map[string]*Actor),
	}
}

// 衍生出一个Actor
func (s *Scheduler) spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	o := defaultActorOptions()
	for _, opt := range opts {
		opt(o)
	}

	act := &Actor{}
	act.opts = o
	act.scheduler = s
	act.state.Store(started)
	act.routes = make(map[int32]RouteHandler)
	act.events = make(map[cluster.Event]EventHandler, 3)
	act.mailbox = make(chan Context, 4096)
	act.fnChan = make(chan func(), 4096)
	act.processor = creator(act, o.args...)

	s.mu.Lock()

	if _, ok := s.load(act.Kind(), act.ID()); ok {
		s.mu.Unlock()
		return nil, errors.ErrActorExists
	}

	act.processor.Init()

	if _, ok := s.kinds.Load(act.Kind()); !ok {
		s.kinds.Store(act.Kind(), struct{}{})

		for route := range act.routes {
			s.routes.Store(route, act.Kind())
		}
	}

	s.actors.Store(act.PID(), act)

	s.mu.Unlock()

	act.dispatch()
	act.processor.Start()

	return act, nil
}

// 杀死Actor
func (s *Scheduler) kill(kind, id string) bool {
	act, ok := s.remove(kind, id)
	if !ok {
		return false
	}

	act.Destroy()

	return true
}

// 移除Actor
func (s *Scheduler) remove(kind, id string) (*Actor, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	act, ok := s.load(kind, id)
	if !ok {
		return nil, false
	}

	s.actors.Delete(act.PID())

	return act, true
}

// 加载Actor
func (s *Scheduler) load(kind, id string) (*Actor, bool) {
	if actor, ok := s.actors.Load(kind + "/" + id); ok {
		return actor.(*Actor), true
	}

	return nil, false
}

// 为用户与Actor建立绑定关系
func (s *Scheduler) bindActor(uid int64, kind, id string) error {
	if uid == 0 {
		return errors.ErrIllegalOperation
	}

	act, ok := s.load(kind, id)
	if !ok {
		return errors.New("actor not found")
	}

	s.rw.Lock()
	defer s.rw.Unlock()

	relations, ok := s.relations[uid]
	if !ok {
		relations = make(map[string]*Actor)
		s.relations[uid] = relations
	}

	relations[act.Kind()] = act

	return nil
}

// 解绑用户与Actor关系
func (s *Scheduler) unbindActor(uid int64, kind string) {
	s.rw.Lock()
	defer s.rw.Unlock()

	if _, ok := s.relations[uid]; ok {
		delete(s.relations[uid], kind)
	}
}

// 获取用户绑定的Actor
func (s *Scheduler) loadActor(uid int64, kind string) (*Actor, bool) {
	s.rw.RLock()
	defer s.rw.RUnlock()

	if relations, ok := s.relations[uid]; ok {
		if act, ok := relations[kind]; ok {
			return act, true
		}
	}

	return nil, false
}

// 分发消息
func (s *Scheduler) dispatch(ctx Context) error {
	if ctx.Kind() == Request {
		return s.dispatchRequest(ctx)
	} else {
		return s.dispatchEvent(ctx)
	}
}

// 分发请求
func (s *Scheduler) dispatchRequest(ctx Context) error {
	uid := ctx.UID()

	if uid == 0 {
		return errors.ErrMissDispatchStrategy
	}

	kind, ok := s.routes.Load(ctx.Route())
	if !ok {
		return errors.ErrUnregisterRoute
	}

	act, ok := s.loadActor(uid, kind.(string))
	if !ok {
		return errors.ErrNotBindActor
	}

	act.Next(ctx)

	return nil
}

// 分发事件
func (s *Scheduler) dispatchEvent(ctx Context) error {
	s.actors.Range(func(_, act any) bool {
		act.(*Actor).Next(ctx)
		return true
	})

	return nil
}
