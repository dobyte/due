package node

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

// RouteHandler is a route handler function.
type RouteHandler func(ctx Context)

// RouteOptions are route options.
type RouteOptions struct {
	// Internal reports whether the route is internal, and it is not internal by default.
	//
	// External routes can carry messages across clients, gateways and nodes, while internal routes
	// can only carry messages between nodes.
	Internal bool

	// Stateful reports whether the route is stateful, and it is not stateful by default.
	//
	// Messages of a stateless route are dispatched to different node servers by the load balancing
	// strategy, while messages of a stateful route are always routed to the node server the user is
	// bound to.
	Stateful bool

	// Authorized reports whether the route is authorized, and it is not authorized by default.
	//
	// An authorized route must carry the UID when it flows across the cluster, otherwise it cannot
	// be delivered. The gateway can use this option to intercept unauthorized connections early and
	// reduce the attack handling pressure on node servers.
	Authorized bool

	// Middlewares are the route middlewares.
	Middlewares []MiddlewareHandler
}

var (
	InternalRoute   = RouteOptions{Internal: true}   // Internal route, messages flow between nodes only
	StatefulRoute   = RouteOptions{Stateful: true}   // Stateful route, always routed to the bound node
	AuthorizedRoute = RouteOptions{Authorized: true} // Authorized route, must carry the UID when flowing
)

// Router is a router.
//
// It carries the routing queue of a node and is responsible for receiving and handling routed
// messages.
type Router struct {
	node                *Node
	mwPool              sync.Pool
	queue               *queue.Queue[*request]
	routes              map[int32]*routeEntity
	preRouteHandler     RouteHandler
	postRouteHandler    RouteHandler
	defaultRouteHandler RouteHandler
}

// routeEntity is a route entity.
type routeEntity struct {
	route   int32        // Route number
	handler RouteHandler // Route handler
	options RouteOptions // Route options
}

// newRouter creates a new router for the given node server.
func newRouter(node *Node) *Router {
	return &Router{
		node:   node,
		queue:  queue.NewQueue[*request](node.opts.messageQueueSize, node.opts.messageWriteTimeout, &sync.RWMutex{}),
		routes: make(map[int32]*routeEntity),
		mwPool: sync.Pool{New: func() any { return &Middleware{} }},
	}
}

// AddRouteHandler adds a route handler.
func (r *Router) AddRouteHandler(route int32, handler RouteHandler, opts ...RouteOptions) {
	if r.node.isShut() {
		if len(opts) > 0 {
			r.routes[route] = &routeEntity{
				route:   route,
				handler: handler,
				options: opts[0],
			}
		} else {
			r.routes[route] = &routeEntity{
				route:   route,
				handler: handler,
			}
		}
	} else {
		log.Warnf("the node server is not shut, can't add route handler")
	}
}

// SetDefaultRouteHandler sets the default route handler. Every unregistered route goes through the
// default route handler.
func (r *Router) SetDefaultRouteHandler(handler RouteHandler) {
	if r.node.isShut() {
		r.defaultRouteHandler = handler
	} else {
		log.Warnf("the node server is not shut, can't set default route handler")
	}
}

// HasDefaultRouteHandler reports whether a default route handler exists.
func (r *Router) HasDefaultRouteHandler() bool {
	return r.defaultRouteHandler != nil
}

// SetPreRouteHandler sets the pre-route handler.
func (r *Router) SetPreRouteHandler(handler RouteHandler) {
	if r.node.isShut() {
		r.preRouteHandler = handler
	} else {
		log.Warnf("the node server is not shut, can't set pre-route handler")
	}
}

// SetPostRouteHandler sets the post-route handler.
func (r *Router) SetPostRouteHandler(handler RouteHandler) {
	if r.node.isShut() {
		r.postRouteHandler = handler
	} else {
		log.Warnf("the node server is not shut, can't set post-route handler")
	}
}

// CheckRouteStateful reports whether the route is stateful and whether the route exists.
func (r *Router) CheckRouteStateful(route int32) (stateful bool, exist bool) {
	if entity, ok := r.routes[route]; ok {
		exist, stateful = ok, entity.options.Stateful
	}
	return
}

// Group groups routes into a RouterGroup configured by the given functions.
func (r *Router) Group(groups ...func(group *RouterGroup)) *RouterGroup {
	group := &RouterGroup{
		router:      r,
		middlewares: make([]MiddlewareHandler, 0),
	}

	for _, fn := range groups {
		fn(group)
	}

	return group
}

// deliver routes a message. It fetches a request object from the pool, fills in the message content
// and writes it to the routing queue for asynchronous handling. It returns the error reported when
// the message fails to be enqueued.
func (r *Router) deliver(gid, nid, pid string, cid, uid int64, seq, route int32, message any) error {
	req := r.node.reqPool.Get().(*request)
	req.gid = gid
	req.nid = nid
	req.pid = pid
	req.cid = cid
	req.uid = uid
	req.seq = seq
	req.route = route
	req.message = message

	if r.node.opts.ctxFunc != nil {
		req.ctx = r.node.opts.ctxFunc()
	} else {
		req.ctx = context.Background()
	}

	if err := r.queue.Write(req); err != nil {
		req.release()
		return err
	}

	return nil
}

// receive returns the channel from which routed messages are received.
func (r *Router) receive() <-chan *request {
	return r.queue.Read()
}

// close closes the router.
func (r *Router) close() {
	r.queue.Close()
}

// clean releases every request object still in the routing queue.
func (r *Router) clean() {
	r.queue.Clean(func(req *request) { req.release() })
}

// handle handles a routed message. It looks up the matching route handler, runs the pre-route and
// post-route handlers and the middleware chain, then recycles the request object once handling is
// done.
func (r *Router) handle(req *request) {
	r.queue.Done(req == nil)

	if req == nil {
		return
	}

	version := req.incrVersion()

	route, ok := r.routes[req.route]
	if !ok && r.defaultRouteHandler == nil {
		req.compareVersionRecycle(version)
		log.Warnf("message routing does not register handler function, route: %v", req.route)
		return
	}

	if r.preRouteHandler != nil {
		xcall.Call(func() { r.preRouteHandler(req) })
	}

	if ok {
		if len(route.options.Middlewares) > 0 {
			middleware := r.mwPool.Get().(*Middleware)
			middleware.index = -1
			middleware.middlewares = route.options.Middlewares
			middleware.routeHandler = route.handler

			middleware.Next(req)

			r.mwPool.Put(middleware)
			return
		} else {
			xcall.Call(func() { route.handler(req) })
		}
	} else {
		xcall.Call(func() { r.defaultRouteHandler(req) })
	}

	req.compareVersionExecDefer(version)

	req.compareVersionRecycle(version)
}

// RouterGroup is a route group.
//
// It manages a set of routes and their middlewares together.
type RouterGroup struct {
	router      *Router
	middlewares []MiddlewareHandler
}

// Middleware adds middlewares to the route group.
func (g *RouterGroup) Middleware(middlewares ...MiddlewareHandler) *RouterGroup {
	g.middlewares = append(g.middlewares, middlewares...)

	return g
}

// AddRouteHandler adds a route handler. It merges the route group middlewares with the middlewares
// of the route itself.
func (g *RouterGroup) AddRouteHandler(route int32, handler RouteHandler, opts ...RouteOptions) *RouterGroup {
	var options RouteOptions

	if len(opts) > 0 {
		options = opts[0]
		options.Middlewares = make([]MiddlewareHandler, len(g.middlewares)+len(opts[0].Middlewares))
		copy(options.Middlewares, g.middlewares)
		copy(options.Middlewares[len(g.middlewares):], opts[0].Middlewares)
	} else {
		options = RouteOptions{}
		options.Middlewares = make([]MiddlewareHandler, len(g.middlewares))
		copy(options.Middlewares, g.middlewares)
	}

	g.router.AddRouteHandler(route, handler, options)

	return g
}
