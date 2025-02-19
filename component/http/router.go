package http

import (
	"github.com/gofiber/fiber/v3"
)

type Handler = func(ctx Context) error

type Router interface {
	// Get 添加GET请求处理器
	Get(path string, handler any, middlewares ...any) Router
	// Post 添加POST请求处理器
	Post(path string, handler any, middlewares ...any) Router
	// Head 添加HEAD请求处理器
	Head(path string, handler any, middlewares ...any) Router
	// Put 添加PUT请求处理器
	Put(path string, handler any, middlewares ...any) Router
	// Delete 添加DELETE请求处理器
	Delete(path string, handler any, middlewares ...any) Router
	// Connect 添加CONNECT请求处理器
	Connect(path string, handler any, middlewares ...any) Router
	// Options 添加OPTIONS请求处理器
	Options(path string, handler any, middlewares ...any) Router
	// Trace 添加TRACE请求处理器
	Trace(path string, handler any, middlewares ...any) Router
	// Patch 添加PATCH请求处理器
	Patch(path string, handler any, middlewares ...any) Router
	// Add 添加路由处理器
	Add(methods []string, path string, handler any, middlewares ...any) Router
	// Group 路由组
	Group(prefix string, middlewares ...any) Router
}

type router struct {
	app   *fiber.App
	proxy *Proxy
}

// Get 添加GET请求处理器
func (r *router) Get(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodGet}, path, handler, middlewares...)
}

// Post 添加POST请求处理器
func (r *router) Post(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodPost}, path, handler, middlewares...)
}

// Head 添加HEAD请求处理器
func (r *router) Head(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodHead}, path, handler, middlewares...)
}

// Put 添加PUT请求处理器
func (r *router) Put(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodPut}, path, handler, middlewares...)
}

// Delete 添加DELETE请求处理器
func (r *router) Delete(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodDelete}, path, handler, middlewares...)
}

// Connect 添加CONNECT请求处理器
func (r *router) Connect(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodConnect}, path, handler, middlewares...)
}

// Options 添加OPTIONS请求处理器
func (r *router) Options(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodOptions}, path, handler, middlewares...)
}

// Trace 添加TRACE请求处理器
func (r *router) Trace(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodTrace}, path, handler, middlewares...)
}

// Patch 添加PATCH请求处理器
func (r *router) Patch(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodPatch}, path, handler, middlewares...)
}

// All 添加任意请求处理器
func (r *router) All(path string, handler any, middlewares ...any) Router {
	return r.Add(fiber.DefaultMethods, path, handler, middlewares...)
}

// Add 添加路由处理器
func (r *router) Add(methods []string, path string, handler any, middlewares ...any) Router {
	handlers := make([]fiber.Handler, 0, len(middlewares))
	for i := range middlewares {
		middleware := middlewares[i]

		switch h := middleware.(type) {
		case fiber.Handler:
			handlers = append(handlers, h)
		case Handler:
			handlers = append(handlers, func(ctx fiber.Ctx) error {
				return h(&context{Ctx: ctx, proxy: r.proxy})
			})
		}
	}

	switch h := handler.(type) {
	case fiber.Handler:
		r.app.Add(methods, path, h, handlers...)
	case Handler:
		r.app.Add(methods, path, func(ctx fiber.Ctx) error {
			return h(&context{Ctx: ctx, proxy: r.proxy})
		}, handlers...)
	}

	return r
}

// Group 路由组
func (r *router) Group(prefix string, middlewares ...any) Router {
	handlers := make([]fiber.Handler, 0, len(middlewares))
	for i := range middlewares {
		middleware := middlewares[i]

		switch h := middleware.(type) {
		case fiber.Handler:
			handlers = append(handlers, h)
		case Handler:
			handlers = append(handlers, func(ctx fiber.Ctx) error {
				return h(&context{Ctx: ctx, proxy: r.proxy})
			})
		}
	}

	return &routeGroup{proxy: r.proxy, router: r.app.Group(prefix, handlers...)}
}

type routeGroup struct {
	proxy  *Proxy
	router fiber.Router
}

// Get 添加GET请求处理器
func (r *routeGroup) Get(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodGet}, path, handler, middlewares...)
}

// Post 添加GET请求处理器
func (r *routeGroup) Post(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodPost}, path, handler, middlewares...)
}

// Head 添加HEAD请求处理器
func (r *routeGroup) Head(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodHead}, path, handler, middlewares...)
}

// Put 添加PUT请求处理器
func (r *routeGroup) Put(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodPut}, path, handler, middlewares...)
}

// Delete 添加DELETE请求处理器
func (r *routeGroup) Delete(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodDelete}, path, handler, middlewares...)
}

// Connect 添加CONNECT请求处理器
func (r *routeGroup) Connect(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodConnect}, path, handler, middlewares...)
}

// Options 添加OPTIONS请求处理器
func (r *routeGroup) Options(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodOptions}, path, handler, middlewares...)
}

// Trace 添加TRACE请求处理器
func (r *routeGroup) Trace(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodTrace}, path, handler, middlewares...)
}

// Patch 添加PATCH请求处理器
func (r *routeGroup) Patch(path string, handler any, middlewares ...any) Router {
	return r.Add([]string{fiber.MethodPatch}, path, handler, middlewares...)
}

// All 添加任意请求处理器
func (r *routeGroup) All(path string, handler any, middlewares ...any) Router {
	return r.Add(fiber.DefaultMethods, path, handler, middlewares...)
}

// Add 添加路由处理器
func (r *routeGroup) Add(methods []string, path string, handler any, middlewares ...any) Router {
	handlers := make([]fiber.Handler, 0, len(middlewares))
	for i := range middlewares {
		middleware := middlewares[i]

		switch h := middleware.(type) {
		case fiber.Handler:
			handlers = append(handlers, h)
		case Handler:
			handlers = append(handlers, func(ctx fiber.Ctx) error {
				return h(&context{Ctx: ctx, proxy: r.proxy})
			})
		}
	}

	switch h := handler.(type) {
	case fiber.Handler:
		r.router.Add(methods, path, h, handlers...)
	case Handler:
		r.router.Add(methods, path, func(ctx fiber.Ctx) error {
			return h(&context{Ctx: ctx, proxy: r.proxy})
		}, handlers...)
	}

	return r
}

// Group 路由组
func (r *routeGroup) Group(prefix string, middlewares ...any) Router {
	handlers := make([]fiber.Handler, 0, len(middlewares))
	for i := range middlewares {
		middleware := middlewares[i]

		switch h := middleware.(type) {
		case fiber.Handler:
			handlers = append(handlers, h)
		case Handler:
			handlers = append(handlers, func(ctx fiber.Ctx) error {
				return h(&context{Ctx: ctx, proxy: r.proxy})
			})
		}
	}

	return &routeGroup{router: r.router.Group(prefix, handlers...), proxy: r.proxy}
}
