package http

import (
	"github.com/gofiber/fiber/v3"
)

type Handler func(ctx Context) error

type Router interface {
	// Get 添加GET请求处理器
	Get(path string, handler Handler, middlewares ...Handler) Router
	// Post 添加GET请求处理器
	Post(path string, handler Handler, middlewares ...Handler) Router
	// Head 添加HEAD请求处理器
	Head(path string, handler Handler, middlewares ...Handler) Router
	// Put 添加PUT请求处理器
	Put(path string, handler Handler, middlewares ...Handler) Router
	// Delete 添加DELETE请求处理器
	Delete(path string, handler Handler, middlewares ...Handler) Router
	// Connect 添加CONNECT请求处理器
	Connect(path string, handler Handler, middlewares ...Handler) Router
	// Options 添加OPTIONS请求处理器
	Options(path string, handler Handler, middlewares ...Handler) Router
	// Trace 添加TRACE请求处理器
	Trace(path string, handler Handler, middlewares ...Handler) Router
	// Patch 添加PATCH请求处理器
	Patch(path string, handler Handler, middlewares ...Handler) Router
	// Add 添加路由处理器
	Add(methods []string, path string, handler Handler, middlewares ...Handler) Router
	// Group 路由组
	Group(prefix string, handlers ...Handler) Router
}

type router struct {
	app *fiber.App
}

// Get 添加GET请求处理器
func (r *router) Get(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodGet}, path, handler, middlewares...)
}

// Post 添加POST请求处理器
func (r *router) Post(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodPost}, path, handler, middlewares...)
}

// Head 添加HEAD请求处理器
func (r *router) Head(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodHead}, path, handler, middlewares...)
}

// Put 添加PUT请求处理器
func (r *router) Put(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodPut}, path, handler, middlewares...)
}

// Delete 添加DELETE请求处理器
func (r *router) Delete(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodDelete}, path, handler, middlewares...)
}

// Connect 添加CONNECT请求处理器
func (r *router) Connect(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodConnect}, path, handler, middlewares...)
}

// Options 添加OPTIONS请求处理器
func (r *router) Options(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodOptions}, path, handler, middlewares...)
}

// Trace 添加TRACE请求处理器
func (r *router) Trace(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodTrace}, path, handler, middlewares...)
}

// Patch 添加PATCH请求处理器
func (r *router) Patch(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodPatch}, path, handler, middlewares...)
}

// All 添加任意请求处理器
func (r *router) All(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add(fiber.DefaultMethods, path, handler, middlewares...)
}

// Add 添加路由处理器
func (r *router) Add(methods []string, path string, handler Handler, middlewares ...Handler) Router {
	handlers := make([]fiber.Handler, 0, len(middlewares))
	for i := range middlewares {
		middleware := middlewares[i]
		handlers = append(handlers, func(ctx fiber.Ctx) error {
			return middleware(&context{Ctx: ctx})
		})
	}

	r.app.Add(methods, path, func(ctx fiber.Ctx) error {
		return handler(&context{Ctx: ctx})
	}, handlers...)
	return r
}

// Group 路由组
func (r *router) Group(prefix string, handlers ...Handler) Router {
	list := make([]fiber.Handler, 0, len(handlers))
	for i := range handlers {
		handler := handlers[i]
		list = append(list, func(ctx fiber.Ctx) error {
			return handler(&context{Ctx: ctx})
		})
	}

	return &routeGroup{router: r.app.Group(prefix)}
}

type routeGroup struct {
	router fiber.Router
}

// Get 添加GET请求处理器
func (r *routeGroup) Get(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodGet}, path, handler, middlewares...)
}

// Post 添加GET请求处理器
func (r *routeGroup) Post(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodPost}, path, handler, middlewares...)
}

// Head 添加HEAD请求处理器
func (r *routeGroup) Head(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodHead}, path, handler, middlewares...)
}

// Put 添加PUT请求处理器
func (r *routeGroup) Put(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodPut}, path, handler, middlewares...)
}

// Delete 添加DELETE请求处理器
func (r *routeGroup) Delete(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodDelete}, path, handler, middlewares...)
}

// Connect 添加CONNECT请求处理器
func (r *routeGroup) Connect(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodConnect}, path, handler, middlewares...)
}

// Options 添加OPTIONS请求处理器
func (r *routeGroup) Options(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodOptions}, path, handler, middlewares...)
}

// Trace 添加TRACE请求处理器
func (r *routeGroup) Trace(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodTrace}, path, handler, middlewares...)
}

// Patch 添加PATCH请求处理器
func (r *routeGroup) Patch(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add([]string{fiber.MethodPatch}, path, handler, middlewares...)
}

// All 添加任意请求处理器
func (r *routeGroup) All(path string, handler Handler, middlewares ...Handler) Router {
	return r.Add(fiber.DefaultMethods, path, handler, middlewares...)
}

// Add 添加路由处理器
func (r *routeGroup) Add(methods []string, path string, handler Handler, middlewares ...Handler) Router {
	handlers := make([]fiber.Handler, 0, len(middlewares))
	for i := range middlewares {
		middleware := middlewares[i]
		handlers = append(handlers, func(ctx fiber.Ctx) error {
			return middleware(&context{Ctx: ctx})
		})
	}

	r.router.Add(methods, path, func(ctx fiber.Ctx) error {
		return handler(&context{Ctx: ctx})
	}, handlers...)
	return r
}

// Group 路由组
func (r *routeGroup) Group(prefix string, handlers ...Handler) Router {
	list := make([]fiber.Handler, 0, len(handlers))
	for i := range handlers {
		handler := handlers[i]
		list = append(list, func(ctx fiber.Ctx) error {
			return handler(&context{Ctx: ctx})
		})
	}

	return &routeGroup{router: r.router.Group(prefix)}
}
