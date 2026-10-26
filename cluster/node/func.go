package node

import "github.com/dobyte/due/v2/utils/xcall"

// safeCallContextHandler invokes fn safely.
//
// It recovers from any panic raised while fn runs: a runtime error ([runtime.Error]) is logged as
// a fatal error, while any other panic is logged together with its error message.
// If fn is nil, it does nothing.
func safeCallContextHandler(fn func(ctx Context), ctx Context) {
	if fn == nil {
		return
	}

	defer xcall.Recover()

	fn(ctx)
}

// safeCallMiddlewareHandler invokes fn safely.
//
// It recovers from any panic raised while fn runs: a runtime error ([runtime.Error]) is logged as
// a fatal error, while any other panic is logged together with its error message.
// If fn is nil, it does nothing.
func safeCallMiddlewareHandler(fn func(*Middleware, Context), m *Middleware, ctx Context) {
	if fn == nil {
		return
	}

	defer xcall.Recover()

	fn(m, ctx)
}
