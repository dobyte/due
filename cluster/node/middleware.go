package node

import "github.com/dobyte/due/v2/utils/xcall"

// MiddlewareHandler is the middleware handler.
type MiddlewareHandler func(middleware *Middleware, ctx Context)

// Middleware is a middleware.
//
// It is used to intercept and process messages uniformly before and after the route handlers run.
type Middleware struct {
	index        int
	middlewares  []MiddlewareHandler
	routeHandler RouteHandler
}

// Next runs the next middleware.
func (m *Middleware) Next(ctx Context) {
	m.Skip(ctx, 1)
}

// Skip skips the given number of middlewares.
//
// It runs the following middlewares in order and, once they are exhausted, runs the final route
// handler.
func (m *Middleware) Skip(ctx Context, skip int) {
	if m.index >= len(m.middlewares) {
		return
	}

	version := ctx.incrVersion()

	ctx.recoverDefer()

	defer func() {
		ctx.compareVersionExecDefer(version)

		ctx.compareVersionRecycle(version)
	}()

	m.index += skip

	xcall.Call(func() {
		if m.index >= len(m.middlewares) {
			m.routeHandler(ctx)
		} else {
			m.middlewares[m.index](m, ctx)
		}
	})
}
