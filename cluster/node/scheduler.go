package node

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"sync"
)

type Scheduler struct {
	node      *Node
	actors    sync.Map
	routes    sync.Map
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
func (s *Scheduler) spawn(creator Creator, opts ...ActorOption) *Actor {
	o := defaultActorOptions()
	for _, opt := range opts {
		opt(o)
	}

	act := &Actor{}
	act.opts = o
	act.scheduler = s
	act.routes = make(map[int32]RouteHandler)
	act.events = make(map[cluster.Event]EventHandler, 3)
	act.mailbox = make(chan Context, 4096)
	act.processor = creator(act, o.args...)
	act.processor.Init()
	s.actors.Store(act.PID(), act)
	act.dispatch()
	act.processor.Start()

	return act
}

// 加载Actor
func (s *Scheduler) loadActor(kind, id string) (*Actor, bool) {
	actor, ok := s.actors.Load(kind + "/" + id)
	return actor.(*Actor), ok
}

// 为用户与Actor建立绑定关系
func (s *Scheduler) bindActor(uid int64, kind, id string) error {
	if uid == 0 {
		return errors.ErrIllegalOperation
	}

	act, ok := s.loadActor(kind, id)
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
func (s *Scheduler) unbindActor(uid int64, kind string) error {
	s.rw.Lock()
	defer s.rw.Unlock()

	if relations, ok := s.relations[uid]; ok {
		delete(relations, kind)
	}

	return nil
}

// 获取用户绑定的Actor
func (s *Scheduler) load(uid int64, kind string) (*Actor, bool) {
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
	uid := ctx.UID()

	if uid == 0 {
		return errors.New("missing dispatch condition")
	}

	kind, ok := s.routes.Load(ctx.Route())
	if !ok {
		return errors.New("unregistered route")
	}

	act, ok := s.load(uid, kind.(string))
	if !ok {
		return errors.New("unbind actor")
	}

	act.Next(ctx)

	return nil
}
