package node

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/component"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/task"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/utils/xnet"
	"sync"
	"time"
)

type EventHandler func(gid string, uid int64)

type Event struct {
	event cluster.Event
	gid   string
	cid   int64
	uid   int64
}

type Node struct {
	component.Base
	opts      *options
	ctx       context.Context
	cancel    context.CancelFunc
	chEvent   chan *Event
	chRequest chan *Request
	events    map[cluster.Event]EventHandler
	router    *Router
	proxy     *Proxy
	instance  *registry.ServiceInstance
	rpc       transport.Server
	state     cluster.State
	ctxPool   sync.Pool
	reqPool   sync.Pool
	evtPool   sync.Pool
}

func NewNode(opts ...Option) *Node {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	n := &Node{}
	n.opts = o
	n.events = make(map[cluster.Event]EventHandler, 3)
	n.chEvent = make(chan *Event, 4096)
	n.chRequest = make(chan *Request, 4096)
	n.router = newRouter()
	n.proxy = newProxy(n)
	n.state = cluster.Shut
	n.ctx, n.cancel = context.WithCancel(o.ctx)
	n.ctxPool.New = func() interface{} { return n.allocateContext() }
	n.reqPool.New = func() interface{} { return n.allocateRequest() }
	n.evtPool.New = func() interface{} { return n.allocateEvent() }

	return n
}

// Name 组件名称
func (n *Node) Name() string {
	return n.opts.name
}

// Init 初始化节点
func (n *Node) Init() {
	if n.opts.id == "" {
		log.Fatal("instance id can not be empty")
	}

	if n.opts.codec == nil {
		log.Fatal("codec component is not injected")
	}

	if n.opts.locator == nil {
		log.Fatal("locator component is not injected")
	}

	if n.opts.registry == nil {
		log.Fatal("registry component is not injected")
	}

	if n.opts.transporter == nil {
		log.Fatal("transporter component is not injected")
	}
}

// Start 启动节点
func (n *Node) Start() {
	n.state = cluster.Work

	n.startTransportServer()

	n.registerServiceInstance()

	n.startEventBus()

	n.proxy.watch(n.ctx)

	go n.dispatch()

	n.debugPrint()
}

// Destroy 销毁网关服务器
func (n *Node) Destroy() {
	n.deregisterServiceInstance()

	n.stopTransportServer()

	n.stopEventBus()

	close(n.chEvent)
	close(n.chRequest)
	n.cancel()
	task.Release()
}

// Proxy 获取节点代理
func (n *Node) Proxy() *Proxy {
	return n.proxy
}

// 分发处理消息
func (n *Node) dispatch() {
	for {
		select {
		case evt, ok := <-n.chEvent:
			if !ok {
				return
			}

			handler, ok := n.events[evt.event]
			if !ok {
				log.Warnf("event does not register handler function, event: %v", evt.event)
				continue
			}

			handler(evt.gid, evt.uid)
			n.evtPool.Put(evt)
		case req, ok := <-n.chRequest:
			if !ok {
				return
			}

			route, ok := n.router.routes[req.Route()]
			if !ok && n.router.defaultRouteHandler == nil {
				log.Warnf("message routing does not register handler function, route: %v", req.Route())
				continue
			}

			ctx := n.ctxPool.Get().(*Context)
			ctx.Request = req

			if ok {
				fn := func() {
					if len(route.middlewares) > 0 {
						ctx.Middleware.reset(route.middlewares)
						ctx.Middleware.Next(ctx)

						if ctx.Middleware.isFinished() {
							route.handler(ctx)
						}
					} else {
						route.handler(ctx)
					}

					n.reqPool.Put(req)
					n.ctxPool.Put(ctx)
				}

				if route.stateful {
					fn()
				} else {
					if err := task.AddTask(fn); err != nil {
						log.Warnf("task add failed, system auto switch to blocking invoke, err: %v", err)
						fn()
					}
				}
			} else {
				n.router.defaultRouteHandler(ctx)
				n.reqPool.Put(req)
				n.ctxPool.Put(ctx)
			}
		}
	}
}

// 启动RPC服务器
func (n *Node) startTransportServer() {
	var err error

	n.rpc, err = n.opts.transporter.NewNodeServer(&provider{n})
	if err != nil {
		log.Fatalf("the transport server build failed: %v", err)
	}

	go func() {
		if err = n.rpc.Start(); err != nil {
			log.Fatalf("the transport server startup failed: %v", err)
		}
	}()
}

// 停止RPC服务器
func (n *Node) stopTransportServer() {
	if err := n.rpc.Stop(); err != nil {
		log.Errorf("the transport server stop failed: %v", err)
	}
}

// 启动事件总线
func (n *Node) startEventBus() {
	if n.opts.eventbus == nil {
		return
	}

	go n.opts.eventbus.Watch()
}

// 停止事件总线
func (n *Node) stopEventBus() {
	if n.opts.eventbus == nil {
		return
	}

	if err := n.opts.eventbus.Stop(); err != nil {
		log.Errorf("the eventbus stop failed: %v", err)
	}
}

// 注册服务实例
func (n *Node) registerServiceInstance() {
	routes := make([]registry.Route, 0, len(n.router.routes))
	for _, entity := range n.router.routes {
		routes = append(routes, registry.Route{
			ID:       entity.route,
			Stateful: entity.stateful,
		})
	}

	n.instance = &registry.ServiceInstance{
		ID:       n.opts.id,
		Name:     string(cluster.Node),
		Kind:     cluster.Node,
		Alias:    n.opts.name,
		State:    n.state,
		Routes:   routes,
		Endpoint: n.rpc.Endpoint().String(),
	}

	ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
	err := n.opts.registry.Register(ctx, n.instance)
	cancel()
	if err != nil {
		log.Fatalf("the node service instance register failed: %v", err)
	}
}

// 解注册服务实例
func (n *Node) deregisterServiceInstance() {
	ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
	err := n.opts.registry.Deregister(ctx, n.instance)
	cancel()
	if err != nil {
		log.Errorf("the node service instance deregister failed: %v", err)
	}
}

// 添加路由处理器
func (n *Node) addRouteHandler(route int32, stateful bool, handler RouteHandler) {
	if n.state == cluster.Shut {
		//n.routes[route] = routeEntity{
		//	route:    route,
		//	stateful: stateful,
		//	handler:  handler,
		//}
	} else {
		log.Warnf("the node server is working, can't add route handler")
	}
}

// 默认路由处理器
func (n *Node) setDefaultRouteHandler(handler RouteHandler) {
	if n.state == cluster.Shut {
		//n.defaultRouteHandler = handler
	} else {
		log.Warnf("the node server is working, can't set default route handler")
	}
}

// 是否为有状态路由
func (n *Node) checkRouteStateful(route int32) (bool, bool) {
	if entity, ok := n.router.routes[route]; ok {
		return entity.stateful, ok
	}

	return false, n.router.defaultRouteHandler != nil
}

// 添加事件处理器
func (n *Node) addEventListener(event cluster.Event, handler EventHandler) {
	if n.state == cluster.Shut {
		n.events[event] = handler
	} else {
		log.Warnf("the node server is working, can't add event handler")
	}
}

// 分配Context
func (n *Node) allocateContext() *Context {
	return &Context{
		ctx:        context.Background(),
		Proxy:      n.proxy,
		Middleware: &Middleware{},
	}
}

// 分配请求
func (n *Node) allocateRequest() *Request {
	return &Request{
		codec:     n.opts.codec,
		decryptor: n.opts.decryptor,
		message:   &Message{},
	}
}

// 分配事件
func (n *Node) allocateEvent() *Event {
	return &Event{}
}

func (n *Node) debugPrint() {
	log.Debugf("The node server startup successful")
	log.Debugf("Transport server, listen: %s protocol: %s", xnet.FulfillAddr(n.rpc.Addr()), n.rpc.Scheme())
}
