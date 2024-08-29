package node

import "sync"

type scheduler struct {
	actors    sync.Map
	routes    sync.Map
	rw        sync.RWMutex
	relations map[int64]map[string]Actor
}

func newScheduler() *scheduler {
	return &scheduler{
		relations: make(map[int64]map[string]Actor),
	}
}

// 为用户与Actor建立绑定关系
func (s *scheduler) bind(uid int64, act Actor) {
	s.rw.Lock()
	defer s.rw.Unlock()

	relations, ok := s.relations[uid]
	if !ok {
		relations = make(map[string]Actor)
		s.relations[uid] = relations
	}

	relations[act.Kind()] = act
}

// 解绑用户与Actor关系
func (s *scheduler) unbind(uid int64, act Actor) {
	s.rw.Lock()
	defer s.rw.Unlock()

	if relations, ok := s.relations[uid]; ok {
		delete(relations, act.Kind())
	}
}

// 获取用户绑定的Actor
func (s *scheduler) load(uid int64, kind string) (Actor, bool) {
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
func (s *scheduler) dispatch(ctx Context) bool {
	kind, ok := s.routes.Load(ctx.Route())
	if !ok {
		return false
	}

	act, ok := s.load(ctx.UID(), kind.(string))
	if !ok {
		return false
	}

	act.deliver(ctx)

	return true
}
