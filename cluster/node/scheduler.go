package node

import (
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

// relationShardNum 用户绑定关系分片数量（2的幂，便于位运算取片）
const relationShardNum = 256

// relationShard 用户绑定关系分片
// 按UID低位散列分片以稀释读写锁竞争；填充至64字节缓存行以消除相邻分片间的伪共享（false sharing）
type relationShard struct {
	rw        sync.RWMutex
	relations map[int64]map[string]*Actor
	_         [32]byte
}

// Scheduler 调度器
// 负责Actor的创建、销毁以及用户与Actor关系的维护和消息分发
type Scheduler struct {
	node   *Node
	rw     sync.RWMutex // 保护actors/kinds/routes生命周期一致性
	actors sync.Map
	routes sync.Map
	kinds  sync.Map
	shards [relationShardNum]relationShard // 用户与Actor绑定关系分片
}

// kindEntity Kind实体
// 维护可调度Actor的引用计数与该Kind已注册的路由集合
// 最后一个Actor释放时按路由集合精准清理调度路由，避免全量遍历路由表
type kindEntity struct {
	count  atomic.Int32 // 该Kind下可调度Actor的引用计数
	routes sync.Map     // 该Kind已注册的路由集合（route int32 → struct{}）
}

// 创建调度器
// @param node *Node 节点服务器
// @return @1 *Scheduler 调度器
func newScheduler(node *Node) *Scheduler {
	s := &Scheduler{node: node}
	for i := range s.shards {
		s.shards[i].relations = make(map[int64]map[string]*Actor)
	}
	return s
}

// 按用户ID定位绑定关系分片
// @param uid int64 用户ID
// @return @1 *relationShard 绑定关系分片
func (s *Scheduler) shard(uid int64) *relationShard {
	return &s.shards[uint64(uid)&(relationShardNum-1)]
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
// 从调度器的Actor表中删除Actor，用户绑定关系由destroy负责清理
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
// 绑定关系按UID散列到独立分片写入，避免全局锁阻塞其他用户的消息分发
// @param uid int64 用户ID
// @param kind string Actor类型
// @param id string Actor编号
// @return @1 error 用户ID非法或Actor不存在/已销毁时返回的错误
func (s *Scheduler) bindActor(uid int64, kind, id string) error {
	if uid == 0 {
		return errors.ErrIllegalOperation
	}

	act, ok := s.load(kind, id)
	if !ok {
		return errors.ErrNotFoundActor
	}

	// 校验Actor是否仍处于启动状态，防止与kill/destroy并发时写入指向已销毁Actor的悬垂关系
	if !act.started() {
		return errors.ErrActorNotStarted
	}

	// 先登记到Actor绑定表再写入调度关系，保证并发destroy遍历binds时能够清理本次绑定
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

	// 二次校验Actor状态：与kill/destroy并发时回收已写入的绑定，避免悬垂关系
	if !act.started() {
		act.unbindUser(uid)
		s.doUnbindActor(uid, act.Kind(), act)
		return errors.ErrActorNotStarted
	}

	return nil
}

// 解绑用户与Actor关系
// @param uid int64 用户ID
// @param kind string Actor类型
// @return @1 error 用户或Actor关系不存在时返回的错误
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

// 解绑Actor的全部用户绑定关系
// 逐用户定位分片加锁清理，避免持全局锁遍历绑定表而阻塞整个节点的消息分发
// @param act *Actor 待解绑的Actor
func (s *Scheduler) unbindAllActor(act *Actor) {
	act.binds.Range(func(k, _ any) bool {
		s.doUnbindActor(k.(int64), act.Kind(), act)

		act.binds.Delete(k)

		return true
	})
}

// 执行解绑用户与Actor关系
// 仅当当前绑定仍指向act时才删除，防止误删用户重新绑定到同Kind新Actor的关系
// @param uid int64 用户ID
// @param kind string Actor类型
// @param act *Actor 期望解绑的Actor
// @return @1 bool 是否成功解绑
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

// 释放Kind引用
// 当某个Kind的最后一个可调度Actor被销毁时，清理该Kind及其路由映射，避免条目累积残留
// @param kind string Actor类型
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

	// 仅清理仍指向当前Kind的路由，避免误删被其他Kind覆盖的路由映射
	entity.routes.Range(func(route, _ any) bool {
		s.routes.CompareAndDelete(route, kind)

		return true
	})
}

// 获取用户绑定的Actor
// 每条下放消息均会调用，按UID分片加读锁，避免全局锁在高并发分发下产生缓存行竞争
// @param uid int64 用户ID
// @param kind string Actor类型
// @return @1 *Actor 用户绑定的Actor实例
// @return @2 bool 是否存在对应的绑定关系
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
