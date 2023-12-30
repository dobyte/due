package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xcall"
	"sync/atomic"
	"time"
	"unsafe"
)

type Node struct {
	component.Base
	opts        *options
	ctx         context.Context
	cancel      context.CancelFunc
	state       cluster.State
	events      *Events
	router      *Router
	proxy       *Proxy
	instance    *registry.ServiceInstance
	transporter transport.Server
	fnChan      chan func()
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
	n.fnChan = make(chan func(), 4096)
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

	if n.opts.name == "" {
		log.Fatal("instance name can not be empty")
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

	n.startTransporter()

	n.registerServiceInstance()

	n.proxy.watch(n.ctx)

	go n.dispatch()

	n.debugPrint()
}

// Destroy 销毁网关服务器
func (n *Node) Destroy() {
	n.deregisterServiceInstance()

	n.stopTransporter()

	n.events.close()

	n.router.close()

	close(n.fnChan)

	n.cancel()
}

// Proxy 获取节点代理
func (n *Node) Proxy() *Proxy {
	return n.proxy
}

// 分发处理消息
func (n *Node) dispatch() {
	for {
		select {
		case evt, ok := <-n.events.receive():
			if !ok {
				return
			}
			xcall.Call(func() {
				n.events.handle(evt)
			})
		case ctx, ok := <-n.router.receive():
			if !ok {
				return
			}
			xcall.Call(func() {
				n.router.handle(ctx)
			})
		case handle, ok := <-n.fnChan:
			if !ok {
				return
			}
			xcall.Call(handle)
		}
	}
}

// 启动传输服务器
func (n *Node) startTransporter() {
	n.opts.transporter.SetDefaultDiscovery(n.opts.registry)

	transporter, err := n.opts.transporter.NewNodeServer(&provider{n})
	if err != nil {
		log.Fatalf("transporter create failed: %v", err)
	}

	n.transporter = transporter

	go func() {
		if err = n.transporter.Start(); err != nil {
			log.Fatalf("transporter start failed: %v", err)
		}
	}()
}

// 停止传输服务器
func (n *Node) stopTransporter() {
	if err := n.transporter.Stop(); err != nil {
		log.Errorf("transporter stop failed: %v", err)
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

	events := make([]int, 0, len(n.events.events))
	for event := range n.events.events {
		events = append(events, int(event))
	}

	n.instance = &registry.ServiceInstance{
		ID:       n.opts.id,
		Name:     string(cluster.Node),
		Kind:     cluster.Node.String(),
		Alias:    n.opts.name,
		State:    n.getState(),
		Routes:   routes,
		Events:   events,
		Endpoint: n.transporter.Endpoint().String(),
	}

	ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
	err := n.opts.registry.Register(ctx, n.instance)
	cancel()
	if err != nil {
		log.Fatalf("register node instance failed: %v", err)
	}
}

// 解注册服务实例
func (n *Node) deregisterServiceInstance() {
	log.Debugf("deregister node instance, alias: %s", n.instance.Alias)

	ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
	err := n.opts.registry.Deregister(ctx, n.instance)
	cancel()
	if err != nil {
		log.Errorf("deregister node instance failed: %v", err)
	}
}

// 设置节点状态
func (n *Node) setState(state cluster.State) {
	if n.checkState(state) {
		return
	}

	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.state)), unsafe.Pointer(&state))

	if n.instance == nil {
		return
	}

	n.instance.State = n.getState()
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
		err := n.opts.registry.Register(ctx, n.instance)
		cancel()
		if err == nil {
			break
		}
	}

	return
}

// 获取节点状态
func (n *Node) getState() string {
	if state := (*string)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&n.state)))); state == nil {
		return cluster.Shut.String()
	} else {
		return *state
	}
}

// 检测节点状态
func (n *Node) checkState(state cluster.State) bool {
	return n.getState() == state.String()
}

func (n *Node) debugPrint() {
	log.Debugf("node server startup successful")
	log.Debugf("%s server listen on %s", n.transporter.Scheme(), n.transporter.Addr())
}
