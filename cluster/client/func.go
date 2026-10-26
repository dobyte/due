package client

import "github.com/dobyte/due/v2/utils/xcall"

// safeCallEventHandler invokes fn safely.
//
// It recovers from any panic raised while fn runs: a runtime error ([runtime.Error]) is logged as
// a fatal error, while any other panic is logged together with its error message.
// If fn is nil, it does nothing.
func safeCallEventHandler(fn func(*Conn), ctx *Conn) {
	if fn == nil {
		return
	}

	defer xcall.Recover()

	fn(ctx)
}

// safeCallRouteHandler invokes fn safely.
//
// It recovers from any panic raised while fn runs: a runtime error ([runtime.Error]) is logged as
// a fatal error, while any other panic is logged together with its error message.
// If fn is nil, it does nothing.
func safeCallRouteHandler(fn func(*Context), ctx *Context) {
	if fn == nil {
		return
	}

	defer xcall.Recover()

	fn(ctx)
}
