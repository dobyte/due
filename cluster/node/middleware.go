package node

type MiddlewareHandler func(ctx *Context)

type Middleware struct {
	index       int
	middlewares []MiddlewareHandler
}

// 重置中间件
func (m *Middleware) reset(middlewares []MiddlewareHandler) {
	m.index = 0
	m.middlewares = middlewares
}

// Next 下一个中间件
func (m *Middleware) Next(ctx *Context) {
	if m.isFinished() {
		return
	}

	m.middlewares[m.index](ctx)
	m.index++
}

// Skip 跳过N个中间件
func (m *Middleware) Skip(ctx *Context, skip int) {
	m.index += skip
	m.Next(ctx)
}

// 是否完成中间件处理
func (m *Middleware) isFinished() bool {
	return m.index >= len(m.middlewares)
}
