package node

import (
	"context"
	"fmt"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xcall"
	"golang.org/x/sync/errgroup"
	"sync"
	"sync/atomic"
)

type HookHandler func(proxy *Proxy)

type serviceEntity struct {
	name     string      // 服务名称;用于定位服务发现
	desc     interface{} // 服务描述(grpc为desc描述对象; rpcx为服务路径)
	provider interface{} // 服务提供者
}

type Node struct {
	component.Base
	opts        *options
	ctx         context.Context
	cancel      context.CancelFunc
	state       atomic.Int32
	evtPool     *sync.Pool
	reqPool     *sync.Pool
	router      *Router
	trigger     *Trigger
	proxy       *Proxy
	services    []*serviceEntity
	instances   []*registry.ServiceInstance
	linker      *node.Server
	fnChan      chan func()
	scheduler   *Scheduler
	transporter transport.Server
	wg          *sync.WaitGroup
	rw          sync.RWMutex
	hooks       map[cluster.Hook][]HookHandler
}

func NewNode(opts ...Option) *Node {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	n := &Node{}
	n.opts = o
	n.ctx, n.cancel = context.WithCancel(o.ctx)
	n.proxy = newProxy(n)
	n.router = newRouter(n)
	n.trigger = newTrigger(n)
	n.scheduler = newScheduler(n)
	n.hooks = make(map[cluster.Hook][]HookHandler)
	n.services = make([]*serviceEntity, 0)
	n.instances = make([]*registry.ServiceInstance, 0)
	n.fnChan = make(chan func(), 4096)
	n.state.Store(int32(cluster.Shut))
	n.wg = &sync.WaitGroup{}
	n.evtPool = &sync.Pool{New: func() interface{} {
		evt := &event{}
		evt.node = n
		evt.actor.Store((*Actor)(nil))

		return evt
	}}
	n.reqPool = &sync.Pool{New: func() interface{} {
		req := &request{}
		req.node = n
		req.message = &cluster.Message{}
		req.actor.Store((*Actor)(nil))

		return req
	}}

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

	n.runHookFunc(cluster.Init)
}

// Start 启动节点
func (n *Node) Start() {
	if !n.state.CompareAndSwap(int32(cluster.Shut), int32(cluster.Work)) {
		return
	}

	n.startLinkServer()

	n.startTransportServer()

	n.registerServiceInstances()

	n.proxy.watch()

	go n.dispatch()

	n.printInfo()

	n.runHookFunc(cluster.Start)
}

// Close 关闭节点
func (n *Node) Close() {
	if !n.state.CompareAndSwap(int32(cluster.Work), int32(cluster.Hang)) {
		if !n.state.CompareAndSwap(int32(cluster.Busy), int32(cluster.Hang)) {
			return
		}
	}

	n.refreshServiceInstances()

	n.runHookFunc(cluster.Close)

	n.wg.Wait()
}

// Destroy 销毁节点服务器
func (n *Node) Destroy() {
	if !n.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
		return
	}

	n.runHookFunc(cluster.Destroy)

	n.deregisterServiceInstances()

	n.stopLinkServer()

	n.stopTransportServer()

	n.router.close()

	n.trigger.close()

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
		case evt, ok := <-n.trigger.receive():
			if !ok {
				return
			}

			n.trigger.handle(evt)
		case req, ok := <-n.router.receive():
			if !ok {
				return
			}

			n.router.handle(req)
		case handle, ok := <-n.fnChan:
			if !ok {
				return
			}

			xcall.Call(handle)

			n.doneWait()
		}
	}
}

// 启动连接服务器
func (n *Node) startLinkServer() {
	linker, err := node.NewServer(n.opts.addr, &provider{node: n})
	if err != nil {
		log.Fatalf("link server create failed: %v", err)
	}

	n.linker = linker

	go func() {
		if err = n.linker.Start(); err != nil {
			log.Fatalf("link server start failed: %v", err)
		}
	}()
}

// 停止连接服务器
func (n *Node) stopLinkServer() {
	if err := n.linker.Stop(); err != nil {
		log.Errorf("link server stop failed: %v", err)
	}
}

// 启动传输服务器
func (n *Node) startTransportServer() {
	if n.opts.transporter == nil {
		return
	}

	n.opts.transporter.SetDefaultDiscovery(n.opts.registry)

	if len(n.services) == 0 {
		return
	}

	transporter, err := n.opts.transporter.NewServer()
	if err != nil {
		log.Fatalf("transport server create failed: %v", err)
	}

	n.transporter = transporter

	for _, entity := range n.services {
		if err = n.transporter.RegisterService(entity.desc, entity.provider); err != nil {
			log.Fatalf("register service failed: %v", err)
		}
	}

	go func() {
		if err = n.transporter.Start(); err != nil {
			log.Fatalf("transport server start failed: %v", err)
		}
	}()
}

// 停止传输服务器
func (n *Node) stopTransportServer() {
	if n.transporter == nil {
		return
	}

	if err := n.transporter.Stop(); err != nil {
		log.Errorf("transport server stop failed: %v", err)
	}
}

// 注册服务实例
func (n *Node) registerServiceInstances() {
	routes := make([]registry.Route, 0, len(n.router.routes))
	events := make([]int, 0, len(n.trigger.events))

	for _, entity := range n.router.routes {
		routes = append(routes, registry.Route{
			ID:       entity.route,
			Stateful: entity.stateful,
			Internal: entity.internal,
		})
	}

	for evt := range n.trigger.events {
		events = append(events, int(evt))
	}

	n.instances = append(n.instances, &registry.ServiceInstance{
		ID:       n.opts.id,
		Name:     cluster.Node.String(),
		Kind:     cluster.Node.String(),
		Alias:    n.opts.name,
		State:    n.getState().String(),
		Routes:   routes,
		Events:   events,
		Endpoint: n.linker.Endpoint().String(),
		Weight:   n.opts.weight,
	})

	if n.transporter != nil {
		services := make([]string, 0, len(n.services))
		for _, item := range n.services {
			services = append(services, item.name)
		}

		n.instances = append(n.instances, &registry.ServiceInstance{
			ID:       n.opts.id,
			Name:     cluster.Mesh.String(),
			Kind:     cluster.Mesh.String(),
			Alias:    n.opts.name,
			State:    n.getState().String(),
			Services: services,
			Endpoint: n.transporter.Endpoint().String(),
			Weight:   n.opts.weight,
		})
	}

	if err := n.doRegisterServiceInstances(); err != nil {
		log.Fatalf("register cluster instances failed: %v", err)
	}
}

// 刷新服务实例状态
func (n *Node) refreshServiceInstances() {
	if err := n.doRefreshServiceInstances(); err != nil {
		log.Errorf("refresh cluster instances failed: %v", err)
	}
}

// 解注册服务实例
func (n *Node) deregisterServiceInstances() {
	eg, ctx := errgroup.WithContext(n.ctx)
	for i := range n.instances {
		instance := n.instances[i]
		eg.Go(func() error {
			ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
			defer cancel()
			return n.opts.registry.Deregister(ctx, instance)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Errorf("deregister cluster instances failed: %v", err)
	}
}

// 执行注册操作
func (n *Node) doRegisterServiceInstances() error {
	eg, ctx := errgroup.WithContext(n.ctx)

	for i := range n.instances {
		instance := n.instances[i]
		eg.Go(func() error {
			ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
			defer cancel()
			return n.opts.registry.Register(ctx, instance)
		})
	}

	return eg.Wait()
}

// 执行刷新实例状态操作
func (n *Node) doRefreshServiceInstances() error {
	for _, instance := range n.instances {
		instance.State = n.getState().String()
	}

	return n.doRegisterServiceInstances()
}

// 获取状态
func (n *Node) getState() cluster.State {
	return cluster.State(n.state.Load())
}

// 更新状态
func (n *Node) setState(state cluster.State) error {
	n.state.Store(int32(state))

	return n.doRefreshServiceInstances()
}

// 执行钩子函数
func (n *Node) runHookFunc(hook cluster.Hook) {
	n.rw.RLock()

	if handlers, ok := n.hooks[hook]; ok {
		wg := &sync.WaitGroup{}
		wg.Add(len(handlers))

		for i := range handlers {
			handler := handlers[i]
			xcall.Go(func() {
				handler(n.proxy)
				wg.Done()
			})
		}

		n.rw.RUnlock()

		wg.Wait()
	} else {
		n.rw.RUnlock()
	}
}

// 添加钩子监听器
func (n *Node) addHookListener(hook cluster.Hook, handler HookHandler) {
	switch hook {
	case cluster.Destroy:
		n.rw.Lock()
		n.hooks[hook] = append(n.hooks[hook], handler)
		n.rw.Unlock()
	default:
		if n.getState() == cluster.Shut {
			n.hooks[hook] = append(n.hooks[hook], handler)
		} else {
			log.Warnf("server is working, can't add hook handler")
		}
	}
}

// 添加服务提供者
func (n *Node) addServiceProvider(name string, desc, provider any) {
	if n.getState() == cluster.Shut {
		n.services = append(n.services, &serviceEntity{
			name:     name,
			desc:     desc,
			provider: provider,
		})
	} else {
		log.Warnf("server is working, can't add service provider")
	}
}

// 打印组件信息
func (n *Node) printInfo() {
	infos := make([]string, 0, 8)
	infos = append(infos, fmt.Sprintf("ID: %s", n.opts.id))
	infos = append(infos, fmt.Sprintf("Name: %s", n.Name()))
	infos = append(infos, fmt.Sprintf("Link: %s", n.linker.ExposeAddr()))
	infos = append(infos, fmt.Sprintf("Codec: %s", n.opts.codec.Name()))
	infos = append(infos, fmt.Sprintf("Locator: %s", n.opts.locator.Name()))
	infos = append(infos, fmt.Sprintf("Registry: %s", n.opts.registry.Name()))

	if n.opts.encryptor != nil {
		infos = append(infos, fmt.Sprintf("Encryptor: %s", n.opts.encryptor.Name()))
	} else {
		infos = append(infos, "Encryptor: -")
	}

	if n.opts.transporter != nil {
		infos = append(infos, fmt.Sprintf("Transporter: %s", n.opts.transporter.Name()))
	} else {
		infos = append(infos, "Transporter: -")
	}

	info.PrintBoxInfo("Node", infos...)
}

func (n *Node) doneWait() {
	if n.getState() != cluster.Shut {
		n.wg.Done()
	}
}

func (n *Node) addWait() {
	if n.getState() != cluster.Shut {
		n.wg.Add(1)
	}
}
