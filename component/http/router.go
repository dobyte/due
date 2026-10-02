package http

import (
	"reflect"

	"net/http"

	"github.com/dobyte/due/v2/log"
	"github.com/gofiber/fiber/v3"
)

type Handler = func(ctx Context) error

// Router accepts the following forms of route registration.
//
// due-style route handlers:
//  1. due.Handler
//
// fiber-style route handlers:
//  1. fiber.Handler
//  2. func(fiber.Ctx)
//
// express-style route handlers:
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
// net/http-style route handlers:
//  1. http.HandlerFunc
//  2. http.Handler
//  3. func(http.ResponseWriter, *http.Request)
//
// fasthttp-style route handlers:
//  1. fasthttp.RequestHandler
//  2. func(*fasthttp.RequestCtx) error
type Router interface {
	// Get registers a handler for GET requests.
	Get(path string, handlers ...any) Router
	// Post registers a handler for POST requests.
	Post(path string, handlers ...any) Router
	// Head registers a handler for HEAD requests.
	Head(path string, handlers ...any) Router
	// Put registers a handler for PUT requests.
	Put(path string, handlers ...any) Router
	// Delete registers a handler for DELETE requests.
	Delete(path string, handlers ...any) Router
	// Connect registers a handler for CONNECT requests.
	Connect(path string, handlers ...any) Router
	// Options registers a handler for OPTIONS requests.
	Options(path string, handlers ...any) Router
	// Trace registers a handler for TRACE requests.
	Trace(path string, handlers ...any) Router
	// Patch registers a handler for PATCH requests.
	Patch(path string, handlers ...any) Router
	// All registers a handler for every method.
	All(path string, handlers ...any) Router
	// Add registers route handlers for the given methods.
	Add(methods []string, path string, handlers ...any) Router
	// Group returns a route group.
	Group(prefix string, middlewares ...any) Router
}

// router is the router implementation.
type router struct {
	app   *fiber.App
	proxy *Proxy
}

// Get registers a handler for GET requests.
//
// path is the route path and handlers are the route handlers. It returns the [Router] for
// chaining.
func (r *router) Get(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodGet}, path, handlers...)
}

// Post registers a handler for POST requests.
//
// path is the route path and handlers are the route handlers. It returns the [Router] for
// chaining.
func (r *router) Post(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPost}, path, handlers...)
}

// Head registers a handler for HEAD requests.
func (r *router) Head(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodHead}, path, handlers...)
}

// Put registers a handler for PUT requests.
func (r *router) Put(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPut}, path, handlers...)
}

// Delete registers a handler for DELETE requests.
//
// path is the route path and handlers are the route handlers. It returns the [Router] for
// chaining.
func (r *router) Delete(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodDelete}, path, handlers...)
}

// Connect registers a handler for CONNECT requests.
func (r *router) Connect(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodConnect}, path, handlers...)
}

// Options registers a handler for OPTIONS requests.
//
// path is the route path and handlers are the route handlers. It returns the [Router] for
// chaining.
func (r *router) Options(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodOptions}, path, handlers...)
}

// Trace registers a handler for TRACE requests.
//
// path is the route path and handlers are the route handlers. It returns the [Router] for
// chaining.
func (r *router) Trace(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodTrace}, path, handlers...)
}

// Patch registers a handler for PATCH requests.
func (r *router) Patch(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPatch}, path, handlers...)
}

// All registers a handler for every method.
func (r *router) All(path string, handlers ...any) Router {
	return r.Add(fiber.DefaultMethods, path, handlers...)
}

// Add registers route handlers for the given methods.
//
// methods is the list of request methods, path is the route path and handlers are the route
// handlers. It returns the [Router] for chaining.
func (r *router) Add(methods []string, path string, handlers ...any) Router {
	if len(handlers) > 0 {
		if handlers = adaptHandlers(handlers); len(handlers) > 0 {
			r.app.Add(methods, path, handlers[0], handlers[1:]...)
		}
	}

	return r
}

// Group returns a route group.
//
// prefix is the route prefix and middlewares are the middlewares. It returns the route group for
// chaining.
func (r *router) Group(prefix string, middlewares ...any) Router {
	return &routeGroup{proxy: r.proxy, router: r.app.Group(prefix, adaptHandlers(middlewares)...)}
}

type routeGroup struct {
	proxy  *Proxy
	router fiber.Router
}

// Get registers a handler for GET requests.
func (r *routeGroup) Get(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodGet}, path, handlers...)
}

// Post registers a handler for POST requests.
//
// path is the route path and handlers are the route handlers. It returns the route group for
// chaining.
func (r *routeGroup) Post(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPost}, path, handlers...)
}

// Head registers a handler for HEAD requests.
func (r *routeGroup) Head(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodHead}, path, handlers...)
}

// Put registers a handler for PUT requests.
//
// path is the route path and handlers are the route handlers. It returns the route group for
// chaining.
func (r *routeGroup) Put(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPut}, path, handlers...)
}

// Delete registers a handler for DELETE requests.
func (r *routeGroup) Delete(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodDelete}, path, handlers...)
}

// Connect registers a handler for CONNECT requests.
//
// path is the route path and handlers are the route handlers. It returns the route group for
// chaining.
func (r *routeGroup) Connect(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodConnect}, path, handlers...)
}

// Options registers a handler for OPTIONS requests.
func (r *routeGroup) Options(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodOptions}, path, handlers...)
}

// Trace registers a handler for TRACE requests.
func (r *routeGroup) Trace(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodTrace}, path, handlers...)
}

// Patch registers a handler for PATCH requests.
func (r *routeGroup) Patch(path string, handlers ...any) Router {
	return r.Add([]string{fiber.MethodPatch}, path, handlers...)
}

// All registers a handler for every method.
//
// path is the route path and handlers are the route handlers. It returns the route group for
// chaining.
func (r *routeGroup) All(path string, handlers ...any) Router {
	return r.Add(fiber.DefaultMethods, path, handlers...)
}

// Add registers route handlers for the given methods.
//
// methods is the list of request methods, path is the route path and handlers are the route
// handlers. It returns the route group for chaining.
func (r *routeGroup) Add(methods []string, path string, handlers ...any) Router {
	if len(handlers) > 0 {
		if handlers = adaptHandlers(handlers); len(handlers) > 0 {
			r.router.Add(methods, path, handlers[0], handlers[1:]...)
		}
	}

	return r
}

// Group returns a route group.
func (r *routeGroup) Group(prefix string, middlewares ...any) Router {
	return &routeGroup{router: r.router.Group(prefix, adaptHandlers(middlewares)...), proxy: r.proxy}
}

// adaptHandlers converts handlers of every supported style into fiber handlers.
func adaptHandlers(handlers []any) []any {
	adaptedHandlers := make([]any, 0, len(handlers))

	for i := range handlers {
		handler := handlers[i]

		rv := reflect.ValueOf(handler)
		rk := rv.Kind()

		if rk == reflect.Func {
			if rv.IsNil() {
				log.Warn("router: skip nil function handler")
				continue
			}
		} else if _, ok := handler.(http.Handler); !ok {
			log.Warn("router: skip non-http handler")
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
