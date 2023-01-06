package node

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/component"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/utils/xnet"
	"time"
)

type RouteHandler func(req Request)

type EventHandler func(gid string, uid int64)

type routeEntity struct {
	route    int32        // 路由
	stateful bool         // 是否有状态
	handler  RouteHandler // 路由处理器
}

type eventEntity struct {
	event cluster.Event
	gid   string
	uid   int64
}

type Node struct {
	component.Base
	opts                *options
	ctx                 context.Context
	cancel              context.CancelFunc
	chEvent             chan *eventEntity
	chRequest           chan *request
	routes              map[int32]routeEntity
	events              map[cluster.Event]EventHandler
	defaultRouteHandler RouteHandler
	proxy               *proxy
	instance            *registry.ServiceInstance
	rpc                 transport.Server
	state               cluster.State
}

func NewNode(opts ...Option) *Node {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	n := &Node{}
	n.opts = o
	n.routes = make(map[int32]routeEntity)
	n.events = make(map[cluster.Event]EventHandler, 3)
	n.chEvent = make(chan *eventEntity, 4096)
	n.chRequest = make(chan *request, 4096)
	n.proxy = newProxy(n)
	n.state = cluster.Shut
	n.ctx, n.cancel = context.WithCancel(o.ctx)

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
		log.Fatal("rpc component is not injected")
	}
}

// Start 启动节点
func (n *Node) Start() {
	n.state = cluster.Work

	n.startTransportServer()

	n.registerServiceInstance()

	n.proxy.watch(n.ctx)

	go n.dispatch()

	n.debugPrint()
}

// Destroy 销毁网关服务器
func (n *Node) Destroy() {
	n.deregisterServiceInstance()

	n.stopTransportServer()

	close(n.chEvent)
	close(n.chRequest)
	n.cancel()
}

// Proxy 获取节点代理
func (n *Node) Proxy() Proxy {
	return n.proxy
}

// 分发处理消息
func (n *Node) dispatch() {
	for {
		select {
		case entity, ok := <-n.chEvent:
			if !ok {
				return
			}

			handler, ok := n.events[entity.event]
			if !ok {
				log.Warnf("event does not register handler function, event: %v", entity.event)
				continue
			}

			handler(entity.gid, entity.uid)
		case req, ok := <-n.chRequest:
			if !ok {
				return
			}

			route, ok := n.routes[req.Route()]
			if ok {
				route.handler(req)
			} else if n.defaultRouteHandler != nil {
				n.defaultRouteHandler(req)
			} else {
				log.Warnf("message routing does not register handler function, route: %v", req.Route())
			}
		}
	}
}

// 启动传输服务器
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

// 注册服务实例
func (n *Node) registerServiceInstance() {
	routes := make([]registry.Route, 0, len(n.routes))
	for _, entity := range n.routes {
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
		n.routes[route] = routeEntity{
			route:    route,
			stateful: stateful,
			handler:  handler,
		}
	} else {
		log.Warnf("the node server is working, can't add route handler")
	}
}

// 默认路由处理器
func (n *Node) setDefaultRouteHandler(handler RouteHandler) {
	if n.state == cluster.Shut {
		n.defaultRouteHandler = handler
	} else {
		log.Warnf("the node server is working, can't set default route handler")
	}
}

// 是否为有状态路由
func (n *Node) checkRouteStateful(route int32) (bool, bool) {
	if entity, ok := n.routes[route]; ok {
		return entity.stateful, ok
	}

	return false, n.defaultRouteHandler != nil
}

// 添加事件处理器
func (n *Node) addEventListener(event cluster.Event, handler EventHandler) {
	if n.state == cluster.Shut {
		n.events[event] = handler
	} else {
		log.Warnf("the node server is working, can't add event handler")
	}
}

// 触发事件
func (n *Node) trigger(event cluster.Event, gid string, uid int64) {
	n.chEvent <- &eventEntity{
		event: event,
		gid:   gid,
		uid:   uid,
	}
}

// 投递消息给当前节点处理
func (n *Node) deliver(req *request) {
	req.ctx = context.Background()
	req.node = n
	n.chRequest <- req
}

func (n *Node) debugPrint() {
	log.Debugf("The node server startup successful")
	log.Debugf("Transport server, listen: %s protocol: %s", xnet.FulfillAddr(n.rpc.Addr()), n.rpc.Scheme())
}
