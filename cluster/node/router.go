package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/log"
	"sync"
)

type RouteHandler func(ctx Context)

type Router struct {
	node                *Node
	routes              map[int32]*routeEntity
	defaultRouteHandler RouteHandler
	reqPool             sync.Pool
	reqChan             chan *request
}

type routeEntity struct {
	route       int32               // 路由
	stateful    bool                // 是否有状态
	internal    bool                // 是否内部路由
	handler     RouteHandler        // 路由处理器
	middlewares []MiddlewareHandler // 路由中间件
}

func newRouter(node *Node) *Router {
	return &Router{
		node:    node,
		routes:  make(map[int32]*routeEntity),
		reqChan: make(chan *request, 4096),
		reqPool: sync.Pool{New: func() interface{} {
			return &request{
				ctx:        context.Background(),
				node:       node,
				message:    &cluster.Message{},
				middleware: &Middleware{},
			}
		}},
	}
}

// AddRouteHandler 添加路由处理器
func (r *Router) AddRouteHandler(route int32, stateful bool, handler RouteHandler, middlewares ...MiddlewareHandler) {
	if r.node.getState() != cluster.Shut {
		log.Warnf("the node server is working, can't add route handler")
		return
	}

	r.routes[route] = &routeEntity{
		route:       route,
		stateful:    stateful,
		handler:     handler,
		middlewares: middlewares[:],
	}
}

// AddInternalRouteHandler 添加内部路由处理器（node节点间路由消息处理）
func (r *Router) AddInternalRouteHandler(route int32, stateful bool, handler RouteHandler, middlewares ...MiddlewareHandler) {
	if r.node.getState() != cluster.Shut {
		log.Warnf("the node server is working, can't add route handler")
		return
	}

	r.routes[route] = &routeEntity{
		route:       route,
		stateful:    stateful,
		internal:    true,
		handler:     handler,
		middlewares: middlewares[:],
	}
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
func (r *Router) SetDefaultRouteHandler(handler RouteHandler) {
	if r.node.getState() != cluster.Shut {
		log.Warnf("the node server is working, can't set default route handler")
		return
	}

	r.defaultRouteHandler = handler
}

// HasDefaultRouteHandler 是否存在默认路由处理器
func (r *Router) HasDefaultRouteHandler() bool {
	return r.defaultRouteHandler != nil
}

// CheckRouteStateful 是否为有状态路由
func (r *Router) CheckRouteStateful(route int32) (stateful bool, exist bool) {
	if entity, ok := r.routes[route]; ok {
		exist, stateful = ok, entity.stateful
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

func (r *Router) deliver(gid, nid string, cid, uid int64, seq, route int32, data interface{}) {
	req := r.reqPool.Get().(*request)
	req.gid = gid
	req.nid = nid
	req.cid = cid
	req.uid = uid
	req.message.Seq = seq
	req.message.Route = route
	req.message.Data = data
	r.reqChan <- req
}

func (r *Router) receive() <-chan *request {
	return r.reqChan
}

func (r *Router) close() {
	close(r.reqChan)
}

func (r *Router) handle(req *request) {
	defer r.reqPool.Put(req)

	route, ok := r.routes[req.message.Route]
	if !ok && r.defaultRouteHandler == nil {
		log.Warnf("message routing does not register handler function, route: %v", req.message.Route)
		return
	}

	if ok {
		if len(route.middlewares) > 0 {
			middleware := &Middleware{
				index:        -1,
				middlewares:  route.middlewares,
				routeHandler: route.handler,
			}
			middleware.Next(req)
		} else {
			route.handler(req)
		}
	} else {
		r.defaultRouteHandler(req)
	}
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
func (g *RouterGroup) AddRouteHandler(route int32, stateful bool, handler RouteHandler, middlewares ...MiddlewareHandler) *RouterGroup {
	dst := make([]MiddlewareHandler, len(g.middlewares)+len(middlewares))
	copy(dst, g.middlewares)
	copy(dst[len(g.middlewares):], middlewares)
	g.router.AddRouteHandler(route, stateful, handler, dst...)

	return g
}

// AddInternalRouteHandler 添加内部路由处理器（node节点间路由消息处理）
func (g *RouterGroup) AddInternalRouteHandler(route int32, stateful bool, handler RouteHandler, middlewares ...MiddlewareHandler) *RouterGroup {
	dst := make([]MiddlewareHandler, len(g.middlewares)+len(middlewares))
	copy(dst, g.middlewares)
	copy(dst[len(g.middlewares):], middlewares)
	g.router.AddInternalRouteHandler(route, stateful, handler, dst...)

	return g
}
