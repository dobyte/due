package node

import (
	"sync"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

// Scheduler 调度器
// 负责Actor的创建、销毁以及用户与Actor关系的维护和消息分发
type Scheduler struct {
	node      *Node
	rw        sync.RWMutex
	actors    sync.Map
	routes    sync.Map
	kinds     sync.Map
	relations map[int64]map[string]*Actor
}

// 创建调度器
// @param node *Node 节点服务器
// @return @1 *Scheduler 调度器
func newScheduler(node *Node) *Scheduler {
	return &Scheduler{
		node:      node,
		relations: make(map[int64]map[string]*Actor),
	}
}

// 衍生出一个Actor
// 创建Actor并初始化处理器，注册到调度器后启动其消息分发
// @param creator Creator Actor处理器创建函数
// @param opts ...ActorOption Actor配置项
// @return @1 *Actor 衍生出的Actor实例
// @return @2 error Actor已存在或创建失败时返回的错误
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

	if o.wait {
		s.node.doAddWait()
	}

	act := &Actor{}
	act.opts = o
	act.scheduler = s
	act.state.Store(started)
	act.routes = make(map[int32]RouteHandler)
	act.events = make(map[cluster.Event]EventHandler, 3)
	act.taskQueue = queue.NewQueue[func()](o.taskQueueSize, o.taskWriteTimeout)
	act.messageQueue = queue.NewQueue[Context](o.messageQueueSize, o.messageWriteTimeout)

	xcall.Call(func() {
		if act.processor = creator(act, o.args...); act.processor != nil {
			act.processor.Init()
		}
	})

	// actor处理器创建失败
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
		if _, ok := s.kinds.Load(act.Kind()); !ok {
			s.kinds.Store(act.Kind(), struct{}{})
			for route := range act.routes {
				s.routes.Store(route, act.Kind())
			}
		}
	}

	s.actors.Store(act.PID(), act)
	s.rw.Unlock()

	xcall.Go(act.dispatch)
	xcall.Call(act.processor.Start)

	return act, nil
}

// 杀死Actor
// @param kind string Actor类型
// @param id string Actor编号
// @return @1 bool 是否成功杀死
func (s *Scheduler) kill(kind, id string) bool {
	if act, ok := s.remove(kind, id); ok {
		return act.destroy()
	} else {
		return false
	}
}

// 移除Actor
// 从调度器中删除Actor并清理其相关绑定关系
// @param kind string Actor类型
// @param id string Actor编号
// @return @1 *Actor 被移除的Actor实例
// @return @2 bool Actor是否存在
func (s *Scheduler) remove(kind, id string) (*Actor, bool) {
	s.rw.Lock()
	defer s.rw.Unlock()

	act, ok := s.load(kind, id)
	if !ok {
		return nil, false
	}

	s.actors.Delete(act.PID())

	for _, relations := range s.relations {
		if a, ok := relations[act.Kind()]; ok && a == act {
			delete(relations, act.Kind())
		}
	}

	return act, true
}

// 加载Actor
// @param kind string Actor类型
// @param id string Actor编号
// @return @1 *Actor Actor实例
// @return @2 bool Actor是否存在
func (s *Scheduler) load(kind, id string) (*Actor, bool) {
	return s.doLoad(kind + "/" + id)
}

// 执行加载Actor
// @param pid string Actor唯一识别ID（Kind/ID）
// @return @1 *Actor Actor实例
// @return @2 bool Actor是否存在
func (s *Scheduler) doLoad(pid string) (*Actor, bool) {
	if actor, ok := s.actors.Load(pid); ok {
		return actor.(*Actor), true
	}

	return nil, false
}

// 为用户与Actor建立绑定关系
// @param uid int64 用户ID
// @param kind string Actor类型
// @param id string Actor编号
// @return @1 error 用户ID非法或Actor不存在时返回的错误
func (s *Scheduler) bindActor(uid int64, kind, id string) error {
	if uid == 0 {
		return errors.ErrIllegalOperation
	}

	act, ok := s.load(kind, id)
	if !ok {
		return errors.ErrNotFoundActor
	}

	act.bindUser(uid)

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
// @param uid int64 用户ID
// @param kind string Actor类型
// @return @1 error 用户或Actor关系不存在时返回的错误
func (s *Scheduler) unbindActor(uid int64, kind string) error {
	s.rw.Lock()
	defer s.rw.Unlock()

	relations, ok := s.relations[uid]
	if !ok {
		return errors.ErrNotFoundActor
	}

	act, ok := relations[kind]
	if !ok {
		return errors.ErrNotFoundActor
	}

	if act.unbindUser(uid) {
		delete(s.relations[uid], kind)
	}

	return nil
}

// 批量解绑Actor
// 在持锁状态下传入全部用户Actor映射，由回调对映射进行批量清理
// @param fn func(relations map[int64]map[string]*Actor) 批量解绑处理函数
func (s *Scheduler) batchUnbindActor(fn func(relations map[int64]map[string]*Actor)) {
	s.rw.Lock()
	fn(s.relations)
	s.rw.Unlock()
}

// 获取用户绑定的Actor
// @param uid int64 用户ID
// @param kind string Actor类型
// @return @1 *Actor 用户绑定的Actor实例
// @return @2 bool 是否存在对应的绑定关系
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
// 根据上下文类型分发给请求处理或事件处理流程
// @param ctx Context 消息上下文
// @return @1 error 分发失败时返回的错误
func (s *Scheduler) dispatch(ctx Context) error {
	if ctx.Kind() == Request {
		return s.dispatchRequest(ctx)
	} else {
		return s.dispatchEvent(ctx)
	}
}

// 分发请求
// 根据路由号定位用户绑定的Actor并投递消息
// @param ctx Context 请求上下文
// @return @1 error 用户ID非法、路由未注册或用户未绑定Actor时返回的错误
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

// 分发事件
// 克隆上下文并投递给所有可被调度的Actor
// @param ctx Context 事件上下文
// @return @1 error 通常返回nil
func (s *Scheduler) dispatchEvent(ctx Context) error {
	s.actors.Range(func(_, actor any) bool {
		if act := actor.(*Actor); act.opts.dispatch {
			c := ctx.Clone()

			if err := act.Next(c); err != nil {
				c.release()
			}
		}

		return true
	})

	return nil
}
