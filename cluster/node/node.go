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
	"sync/atomic"
	"time"
	"unsafe"
)

type Node struct {
	component.Base
	opts     *options
	ctx      context.Context
	cancel   context.CancelFunc
	state    cluster.State
	events   *Events
	router   *Router
	proxy    *Proxy
	instance *registry.ServiceInstance
	rpc      transport.Server
}

func NewNode(opts ...Option) *Node {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	n := &Node{}
	n.opts = o
	n.events = newEvents(n)
	n.router = newRouter(n)
	n.proxy = newProxy(n)
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
		log.Fatal("transporter component is not injected")
	}
}

// Start 启动节点
func (n *Node) Start() {
	n.setState(cluster.Work)

	n.startRPCServer()

	n.registerServiceInstance()

	n.startEventBus()

	n.proxy.watch(n.ctx)

	go n.dispatch()

	n.debugPrint()
}

// Destroy 销毁网关服务器
func (n *Node) Destroy() {
	n.deregisterServiceInstance()

	n.stopRPCServer()

	n.stopEventBus()

	n.events.close()

	n.router.close()

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
		case evt, ok := <-n.events.event():
			if !ok {
				return
			}

			n.events.handle(evt)
		case req, ok := <-n.router.receive():
			if !ok {
				return
			}

			n.router.handle(req)
		}
	}
}

// 启动RPC服务器
func (n *Node) startRPCServer() {
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
func (n *Node) stopRPCServer() {
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
		State:    n.getState(),
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
	//if n.state == cluster.Shut {
	//	//n.routes[route] = routeEntity{
	//	//	route:    route,
	//	//	stateful: stateful,
	//	//	handler:  handler,
	//	//}
	//} else {
	//	log.Warnf("the node server is working, can't add route handler")
	//}
}

// 默认路由处理器
func (n *Node) setDefaultRouteHandler(handler RouteHandler) {
	//if n.state == cluster.Shut {
	//	//n.defaultRouteHandler = handler
	//} else {
	//	log.Warnf("the node server is working, can't set default route handler")
	//}
}

// 设置节点状态
func (n *Node) setState(state cluster.State) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.state)), unsafe.Pointer(&state))
}

// 获取节点状态
func (n *Node) getState() cluster.State {
	if state := (*cluster.State)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&n.state)))); state == nil {
		return cluster.Shut
	} else {
		return *state
	}
}

func (n *Node) debugPrint() {
	log.Debugf("The node server startup successful")
	log.Debugf("Transport server, listen: %s protocol: %s", xnet.FulfillAddr(n.rpc.Addr()), n.rpc.Scheme())
}
