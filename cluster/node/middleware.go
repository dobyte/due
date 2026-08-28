package node

// MiddlewareHandler 中间件处理函数
type MiddlewareHandler func(middleware *Middleware, ctx Context)

// Middleware 中间件
// 用于在路由处理器执行前后进行统一拦截处理
type Middleware struct {
	index        int
	middlewares  []MiddlewareHandler
	routeHandler RouteHandler
}

// Next 下一个中间件
// @param ctx Context 消息上下文
func (m *Middleware) Next(ctx Context) {
	m.Skip(ctx, 1)
}

// Skip 跳过N个中间件
// 依次执行后续中间件，耗尽后执行最终的路由处理器
// @param ctx Context 消息上下文
// @param skip int 跳过的中间件数量
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
