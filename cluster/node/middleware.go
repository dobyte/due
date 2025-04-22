package node

type MiddlewareHandler func(middleware *Middleware, ctx Context)

type Middleware struct {
	index        int
	middlewares  []MiddlewareHandler
	routeHandler RouteHandler
}

// 重置中间件
func (m *Middleware) reset(middlewares []MiddlewareHandler, routeHandler RouteHandler) {
	m.index = -1
	m.middlewares = middlewares
	m.routeHandler = routeHandler
}

// Next 下一个中间件
func (m *Middleware) Next(ctx Context) {
	m.Skip(ctx, 1)
}

// Skip 跳过N个中间件
func (m *Middleware) Skip(ctx Context, skip int) {
	if m.index >= len(m.middlewares) {
		return
	}

	version := ctx.incrVersion()

	ctx.Cancel()

	defer func() {
		ctx.compareVersionExecDefer(version)

		ctx.compareVersionRecycle(version)
	}()

	m.index += skip

	if m.index >= len(m.middlewares) {
		m.routeHandler(ctx)
	} else {
		m.middlewares[m.index](m, ctx)
	}
}
