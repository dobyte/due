package node

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/task"
	"sync"
)

type RouteHandler func(ctx *Context)

type Router struct {
	node                *Node
	routes              map[int32]*routeEntity
	defaultRouteHandler RouteHandler
	chRequest           chan *Request
	ctxPool             sync.Pool
	reqPool             sync.Pool
}

type routeEntity struct {
	route       int32               // 路由
	stateful    bool                // 是否有状态
	handler     RouteHandler        // 路由处理器
	middlewares []MiddlewareHandler // 路由中间件
}

func newRouter(node *Node) *Router {
	return &Router{
		node:      node,
		routes:    make(map[int32]*routeEntity),
		chRequest: make(chan *Request, 4096),
		ctxPool: sync.Pool{New: func() interface{} {
			return &Context{
				ctx:        context.Background(),
				Proxy:      node.proxy,
				Middleware: &Middleware{},
			}
		}},
		reqPool: sync.Pool{New: func() interface{} {
			return &Request{
				node:    node,
				message: &Message{},
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
	req := r.reqPool.Get().(*Request)
	req.gid = gid
	req.nid = nid
	req.cid = cid
	req.uid = uid
	req.message.Seq = seq
	req.message.Route = route
	req.message.Data = data
	r.chRequest <- req
}

func (r *Router) receive() <-chan *Request {
	return r.chRequest
}

func (r *Router) close() {
	close(r.chRequest)
}

func (r *Router) handle(req *Request) {
	route, ok := r.routes[req.Route()]
	if !ok && r.defaultRouteHandler == nil {
		r.reqPool.Put(req)
		log.Warnf("message routing does not register handler function, route: %v", req.Route())
		return
	}

	ctx := r.ctxPool.Get().(*Context)
	ctx.Request = req

	if ok {
		fn := func() {
			if len(route.middlewares) > 0 {
				ctx.Middleware.reset(route.middlewares)
				ctx.Middleware.Next(ctx)

				if ctx.Middleware.isFinished() {
					route.handler(ctx)
				}
			} else {
				route.handler(ctx)
			}

			r.reqPool.Put(req)
			r.ctxPool.Put(ctx)
		}

		if route.stateful {
			fn()
		} else {
			if err := task.AddTask(fn); err != nil {
				log.Warnf("task add failed, system auto switch to blocking invoke, err: %v", err)
				fn()
			}
		}
	} else {
		r.defaultRouteHandler(ctx)
		r.reqPool.Put(req)
		r.ctxPool.Put(ctx)
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
