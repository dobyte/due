package node

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

type RouteHandler func(ctx Context)

// RouteOptions 路由选项
type RouteOptions struct {
	// 是否内部的路由，默认非内部
	// 外部路由可在客户端、网关、节点间进行消息流转
	// 内部路由仅限于在节点间进行消息流转
	Internal bool

	// 是否有状态路由，默认无状态
	// 无状态路由消息会根据负载均衡策略分配到不同的节点服务器进行处理
	// 有状态路由消息会在绑定节点服务器后，固定路由到绑定的节点服务器进行处理
	Stateful bool

	// 是否授权路由，默认非授权
	// 授权路由在集群间流转时必需附带UID信息，否则无法进行路由投递
	// 该参数可在网关层对未授权连接进行提前拦截，降低节点服对于攻击处理的压力
	Authorized bool

	// 路由中间件
	Middlewares []MiddlewareHandler
}

var (
	InternalRoute   = RouteOptions{Internal: true}
	StatefulRoute   = RouteOptions{Stateful: true}
	AuthorizedRoute = RouteOptions{Authorized: true}
)

type Router struct {
	node                *Node
	rw                  sync.RWMutex
	queue               *queue.Queue[*request]
	routes              map[int32]*routeEntity
	preRouteHandler     RouteHandler
	postRouteHandler    RouteHandler
	defaultRouteHandler RouteHandler
}

type routeEntity struct {
	route   int32        // 路由
	handler RouteHandler // 路由处理器
	options RouteOptions // 路由选项
}

func newRouter(node *Node) *Router {
	return &Router{
		node:   node,
		queue:  queue.NewQueue[*request](node.opts.messageQueueSize, node.opts.messageWriteTimeout),
		routes: make(map[int32]*routeEntity),
	}
}

// AddRouteHandler 添加路由处理器
func (r *Router) AddRouteHandler(route int32, handler RouteHandler, opts ...RouteOptions) {
	if r.node.isShut() {
		if len(opts) > 0 {
			r.routes[route] = &routeEntity{
				route:   route,
				handler: handler,
				options: opts[0],
			}
		} else {
			r.routes[route] = &routeEntity{
				route:   route,
				handler: handler,
			}
		}
	} else {
		log.Warnf("the node server is not shut, can't add route handler")
	}
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
func (r *Router) SetDefaultRouteHandler(handler RouteHandler) {
	if r.node.isShut() {
		r.defaultRouteHandler = handler
	} else {
		log.Warnf("the node server is not shut, can't set default route handler")
	}
}

// HasDefaultRouteHandler 是否存在默认路由处理器
func (r *Router) HasDefaultRouteHandler() bool {
	return r.defaultRouteHandler != nil
}

// SetPreRouteHandler 设置前置路由处理器
func (r *Router) SetPreRouteHandler(handler RouteHandler) {
	if r.node.isShut() {
		r.preRouteHandler = handler
	} else {
		log.Warnf("the node server is not shut, can't set pre-route handler")
	}
}

// SetPostRouteHandler 设置后置路由处理器
func (r *Router) SetPostRouteHandler(handler RouteHandler) {
	if r.node.isShut() {
		r.postRouteHandler = handler
	} else {
		log.Warnf("the node server is not shut, can't set post-route handler")
	}
}

// CheckRouteStateful 是否为有状态路由
func (r *Router) CheckRouteStateful(route int32) (stateful bool, exist bool) {
	if entity, ok := r.routes[route]; ok {
		exist, stateful = ok, entity.options.Stateful
	}
	return
}

// Group 路由组
func (r *Router) Group(groups ...func(group *RouterGroup)) *RouterGroup {
	group := &RouterGroup{
		router:      r,
		middlewares: make([]MiddlewareHandler, 0),
	}

	for _, fn := range groups {
		fn(group)
	}

	return group
}

// 投递路由消息
func (r *Router) deliver(gid, nid, pid string, cid, uid int64, seq, route int32, data any) error {
	req := r.node.reqPool.Get().(*request)
	req.gid = gid
	req.nid = nid
	req.pid = pid
	req.cid = cid
	req.uid = uid
	req.message.Seq = seq
	req.message.Route = route
	req.message.Data = data

	if r.node.opts.ctxFunc != nil {
		req.ctx = r.node.opts.ctxFunc()
	} else {
		req.ctx = context.Background()
	}

	r.rw.RLock()
	err := r.queue.Write(req)
	r.rw.RUnlock()
	if err != nil {
		req.release()
		return err
	}

	return nil
}

// 接收路由消息
func (r *Router) receive() <-chan *request {
	return r.queue.Read()
}

// 停止接收事件
func (r *Router) done() error {
	return r.queue.Write(nil)
}

// 等待所有事件完成
func (r *Router) wait() {
	r.queue.Wait()
}

// 关闭路由器
func (r *Router) close() {
	r.rw.Lock()
	r.queue.Close()
	r.rw.Unlock()

	clear(r.routes)
}

// 处理路由消息
func (r *Router) handle(req *request) {
	r.queue.Done(req == nil)

	if req == nil {
		return
	}

	version := req.incrVersion()

	route, ok := r.routes[req.message.Route]
	if !ok && r.defaultRouteHandler == nil {
		req.compareVersionRecycle(version)
		log.Warnf("message routing does not register handler function, route: %v", req.message.Route)
		return
	}

	if r.preRouteHandler != nil {
		xcall.Call(func() { r.preRouteHandler(req) })
	}

	if ok {
		if len(route.options.Middlewares) > 0 {
			middleware := &Middleware{index: -1, middlewares: route.options.Middlewares, routeHandler: route.handler}
			middleware.Next(req)
			return
		} else {
			xcall.Call(func() { route.handler(req) })
		}
	} else {
		xcall.Call(func() { r.defaultRouteHandler(req) })
	}

	req.compareVersionExecDefer(version)

	req.compareVersionRecycle(version)
}

type RouterGroup struct {
	router      *Router
	middlewares []MiddlewareHandler
}

// Middleware 添加中间件
func (g *RouterGroup) Middleware(middlewares ...MiddlewareHandler) *RouterGroup {
	g.middlewares = append(g.middlewares, middlewares...)

	return g
}

// AddRouteHandler 添加路由处理器
func (g *RouterGroup) AddRouteHandler(route int32, handler RouteHandler, opts ...RouteOptions) *RouterGroup {
	var options RouteOptions

	if len(opts) > 0 {
		options = opts[0]
		options.Middlewares = make([]MiddlewareHandler, len(g.middlewares)+len(opts[0].Middlewares))
		copy(options.Middlewares, g.middlewares)
		copy(options.Middlewares[len(g.middlewares):], opts[0].Middlewares)
	} else {
		options = RouteOptions{}
		options.Middlewares = make([]MiddlewareHandler, len(g.middlewares))
		copy(options.Middlewares, g.middlewares)
	}

	g.router.AddRouteHandler(route, handler, options)

	return g
}
