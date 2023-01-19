package node

type RouteHandler func(ctx *Context)

type Router struct {
	routes              map[int32]*routeEntity
	defaultRouteHandler RouteHandler
}

type routeEntity struct {
	route       int32               // 路由
	stateful    bool                // 是否有状态
	handler     RouteHandler        // 路由处理器
	middlewares []MiddlewareHandler // 路由中间件
}

func newRouter() *Router {
	return &Router{routes: make(map[int32]*routeEntity)}
}

// AddRouteHandler 添加路由处理器
func (r *Router) AddRouteHandler(route int32, stateful bool, handler RouteHandler, middlewares ...MiddlewareHandler) {
	r.routes[route] = &routeEntity{
		route:       route,
		stateful:    stateful,
		handler:     handler,
		middlewares: middlewares[:],
	}
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
func (r *Router) SetDefaultRouteHandler(handler RouteHandler) {
	r.defaultRouteHandler = handler
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
