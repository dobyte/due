package http

import (
	"reflect"

	"github.com/gofiber/fiber/v3"
)

type Handler = func(ctx Context) error

// Router支持以下路由注册方式：
//
// 支持due风格路由处理器
//  1. due.Handler
//
// 支持fiber风格路由处理器
//  1. fiber.Handler
//  2. func(fiber.Ctx)
//
// 支持express风格路由处理器
//  1. func(fiber.Req, fiber.Res) error
//  2. func(fiber.Req, fiber.Res)
//  3. func(fiber.Req, fiber.Res, func() error) error
//  4. func(fiber.Req, fiber.Res, func() error)
//  5. func(fiber.Req, fiber.Res, func()) error
//  6. func(fiber.Req, fiber.Res, func())
//  7. func(fiber.Req, fiber.Res, func(error))
//  8. func(fiber.Req, fiber.Res, func(error)) error
//  9. func(fiber.Req, fiber.Res, func(error) error)
//  10. func(fiber.Req, fiber.Res, func(error) error) error
//
// 支持net/http风格路由处理器
//  1. http.HandlerFunc
//  2. http.Handler
//  3. func(http.ResponseWriter, *http.Request)
//
// 支持fasthttp风格路由处理器
//  1. fasthttp.RequestHandler
//  2. func(*fasthttp.RequestCtx) error
type Router interface {
	// Get 添加GET请求处理器
	Get(path string, handlers ...any) Router
	// Post 添加POST请求处理器
	Post(path string, handlers ...any) Router
	// Head 添加HEAD请求处理器
	Head(path string, handlers ...any) Router
	// Put 添加PUT请求处理器
	Put(path string, handlers ...any) Router
	// Delete 添加DELETE请求处理器
	Delete(path string, handlers ...any) Router
	// Connect 添加CONNECT请求处理器
	Connect(path string, handlers ...any) Router
	// Options 添加OPTIONS请求处理器
	Options(path string, handlers ...any) Router
	// Trace 添加TRACE请求处理器
	Trace(path string, handlers ...any) Router
	// Patch 添加PATCH请求处理器
	Patch(path string, handlers ...any) Router
	// All 添加任意请求处理器
	All(path string, handlers ...any) Router
	// Add 添加路由处理器
	Add(methods []string, path string, handlers ...any) Router
	// Group 路由组
	Group(prefix string, middlewares ...any) Router
}

type router struct {
	app   *fiber.App
	proxy *Proxy
}

// Get 添加GET请求处理器
func (r *router) Get(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodGet}, path, handlers...)
}

// Post 添加POST请求处理器
func (r *router) Post(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPost}, path, handlers...)
}

// Head 添加HEAD请求处理器
func (r *router) Head(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodHead}, path, handlers...)
}

// Put 添加PUT请求处理器
func (r *router) Put(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPut}, path, handlers...)
}

// Delete 添加DELETE请求处理器
func (r *router) Delete(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodDelete}, path, handlers...)
}

// Connect 添加CONNECT请求处理器
func (r *router) Connect(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodConnect}, path, handlers...)
}

// Options 添加OPTIONS请求处理器
func (r *router) Options(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodOptions}, path, handlers...)
}

// Trace 添加TRACE请求处理器
func (r *router) Trace(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodTrace}, path, handlers...)
}

// Patch 添加PATCH请求处理器
func (r *router) Patch(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPatch}, path, handlers...)
}

// All 添加任意请求处理器
func (r *router) All(path string, handlers ...any) Router {
	return r.Add(fiber.DefaultMethods, path, handlers...)
}

// Add 添加路由处理器
func (r *router) Add(methods []string, path string, handlers ...any) Router {
	if len(handlers) > 0 {
		if handlers = adaptHandlers(handlers); len(handlers) > 0 {
			r.app.Add(methods, path, handlers[0], handlers[1:]...)
		}
	}

	return r
}

// Group 路由组
func (r *router) Group(prefix string, middlewares ...any) Router {
	return &routeGroup{proxy: r.proxy, router: r.app.Group(prefix, adaptHandlers(middlewares)...)}
}

type routeGroup struct {
	proxy  *Proxy
	router fiber.Router
}

// Get 添加GET请求处理器
func (r *routeGroup) Get(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodGet}, path, handlers...)
}

// Post 添加POST请求处理器
func (r *routeGroup) Post(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPost}, path, handlers...)
}

// Head 添加HEAD请求处理器
func (r *routeGroup) Head(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodHead}, path, handlers...)
}

// Put 添加PUT请求处理器
func (r *routeGroup) Put(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPut}, path, handlers...)
}

// Delete 添加DELETE请求处理器
func (r *routeGroup) Delete(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodDelete}, path, handlers...)
}

// Connect 添加CONNECT请求处理器
func (r *routeGroup) Connect(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodConnect}, path, handlers...)
}

// Options 添加OPTIONS请求处理器
func (r *routeGroup) Options(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodOptions}, path, handlers...)
}

// Trace 添加TRACE请求处理器
func (r *routeGroup) Trace(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodTrace}, path, handlers...)
}

// Patch 添加PATCH请求处理器
func (r *routeGroup) Patch(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPatch}, path, handlers...)
}

// All 添加任意请求处理器
func (r *routeGroup) All(path string, handlers ...any) Router {
	return r.Add(fiber.DefaultMethods, path, handlers...)
}

// Add 添加路由处理器
func (r *routeGroup) Add(methods []string, path string, handlers ...any) Router {
	if len(handlers) > 0 {
		if handlers = adaptHandlers(handlers); len(handlers) > 0 {
			r.router.Add(methods, path, handlers[0], handlers[1:]...)
		}
	}

	return r
}

// Group 路由组
func (r *routeGroup) Group(prefix string, middlewares ...any) Router {
	return &routeGroup{router: r.router.Group(prefix, adaptHandlers(middlewares)...), proxy: r.proxy}
}

// 适配处理器
func adaptHandlers(handlers []any) []any {
	adaptedHandlers := make([]any, 0, len(handlers))

	for i := range handlers {
		handler := handlers[i]

		if reflect.ValueOf(handler).IsNil() {
			continue
		}

		if h, ok := handler.(Handler); ok {
			adaptedHandlers = append(adaptedHandlers, func(ctx fiber.Ctx) error {
				return h(ctx.(Context))
			})
		} else {
			adaptedHandlers = append(adaptedHandlers, handler)
		}
	}

	return adaptedHandlers
}
