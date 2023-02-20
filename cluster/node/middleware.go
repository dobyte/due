package node

type MiddlewareHandler func(ctx *Context)

type Middleware struct {
	index       int
	middlewares []MiddlewareHandler
}

// 重置中间件
func (m *Middleware) reset(middlewares []MiddlewareHandler) {
	m.index = -1
	m.middlewares = middlewares
}

// Next 下一个中间件
func (m *Middleware) Next(ctx *Context) {
	m.Skip(ctx, 1)
}

// Skip 跳过N个中间件
func (m *Middleware) Skip(ctx *Context, skip int) {
	m.index += skip
	if m.isFinished() {
		return
	}
	m.middlewares[m.index](ctx)
}

// 是否完成中间件处理
func (m *Middleware) isFinished() bool {
	return m.index >= len(m.middlewares)
}
