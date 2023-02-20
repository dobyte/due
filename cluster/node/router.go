package node

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/log"
	"sync"
)

type RouteHandler func(ctx *Context)

type Router struct {
	node                *Node
	routes              map[int32]*routeEntity
	defaultRouteHandler RouteHandler
	ctxPool             sync.Pool
	ctxChan             chan *Context
}

type routeEntity struct {
	route       int32               // 路由
	stateful    bool                // 是否有状态
	handler     RouteHandler        // 路由处理器
	middlewares []MiddlewareHandler // 路由中间件
}

func newRouter(node *Node) *Router {
	return &Router{
		node:    node,
		routes:  make(map[int32]*routeEntity),
		ctxChan: make(chan *Context, 4096),
		ctxPool: sync.Pool{New: func() interface{} {
			return &Context{
				ctx:        context.Background(),
				Proxy:      node.proxy,
				Request:    &Request{node: node, Message: &Message{}},
				Middleware: &Middleware{},
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
	ctx := r.ctxPool.Get().(*Context)
	ctx.Request.GID = gid
	ctx.Request.NID = nid
	ctx.Request.CID = cid
	ctx.Request.UID = uid
	ctx.Request.Message.Seq = seq
	ctx.Request.Message.Route = route
	ctx.Request.Message.Data = data
	r.ctxChan <- ctx
}

func (r *Router) receive() <-chan *Context {
	return r.ctxChan
}

func (r *Router) close() {
	close(r.ctxChan)
}

func (r *Router) handle(ctx *Context) {
	defer r.ctxPool.Put(ctx)

	route, ok := r.routes[ctx.Request.Message.Route]
	if !ok && r.defaultRouteHandler == nil {
		log.Warnf("message routing does not register handler function, route: %v", ctx.Request.Message.Route)
		return
	}

	if ok {
		if len(route.middlewares) > 0 {
			ctx.Middleware.reset(route.middlewares)
			ctx.Middleware.Next(ctx)

			if ctx.Middleware.isFinished() {
				route.handler(ctx)
			}
		} else {
			route.handler(ctx)
		}
	} else {
		r.defaultRouteHandler(ctx)
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
