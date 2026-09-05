package node

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/petermattis/goid"
	"golang.org/x/sync/errgroup"
)

// HookHandler 节点钩子处理函数
type HookHandler func(proxy *Proxy)

// 服务实体
type serviceEntity struct {
	name     string // 服务名称;用于定位服务发现
	desc     any    // 服务描述(grpc为desc描述对象; rpcx为服务路径)
	provider any    // 服务提供者
}

// Node 节点服务器
type Node struct {
	component.Base
	opts         *options
	ctx          context.Context
	cancel       context.CancelFunc
	state        atomic.Int32
	evtPool      *sync.Pool
	reqPool      *sync.Pool
	tasker       *Tasker
	router       *Router
	trigger      *Trigger
	proxy        *Proxy
	services     []*serviceEntity
	instances    []*registry.ServiceInstance
	linker       *node.Server
	scheduler    *Scheduler
	transporter  transport.Server
	wg           *sync.WaitGroup
	rw           sync.RWMutex
	hooks        map[cluster.Hook][]HookHandler
	dispatchGoid atomic.Int64
}

// NewNode 创建节点服务器
// 创建后会初始化代理、任务器、路由器、事件触发器与调度器等内部组件，并处于关闭状态
// @param opts ...Option 节点配置项
// @return @1 *Node 节点服务器实例
func NewNode(opts ...Option) *Node {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	n := &Node{}
	n.opts = o
	n.ctx, n.cancel = context.WithCancel(o.ctx)
	n.proxy = newProxy(n)
	n.tasker = newTasker(n)
	n.router = newRouter(n)
	n.trigger = newTrigger(n)
	n.scheduler = newScheduler(n)
	n.hooks = make(map[cluster.Hook][]HookHandler)
	n.services = make([]*serviceEntity, 0)
	n.instances = make([]*registry.ServiceInstance, 0)
	n.state.Store(int32(cluster.Shut))
	n.wg = &sync.WaitGroup{}
	n.evtPool = &sync.Pool{New: func() any {
		evt := &event{}
		evt.node = n
		evt.actor.Store((*Actor)(nil))

		return evt
	}}
	n.reqPool = &sync.Pool{New: func() any {
		req := &request{}
		req.node = n
		req.message = &cluster.Message{}
		req.actor.Store((*Actor)(nil))

		return req
	}}

	return n
}

// Name 组件名称
// @return @1 string 组件名称
func (n *Node) Name() string {
	return n.opts.name
}

// Init 初始化节点
// 校验实例ID、实例名称、编解码器、定位器与注册器等必要配置，缺失时直接终止进程
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
// 将状态置为工作中，随后依次启动连接服务器、传输服务器、注册服务实例并开启消息分发
func (n *Node) Start() {
	if !n.state.CompareAndSwap(int32(cluster.Shut), int32(cluster.Work)) {
		return
	}

	n.startLinkerServer()

	n.startTransportServer()

	n.registerServiceInstances()

	n.proxy.watch()

	go n.dispatch()

	n.printInfo()

	n.runHookFunc(cluster.Start)
}

// Close 关闭节点
// 将状态置为挂起，停止接收新消息并等待任务、路由与事件队列中的存量消息处理完成
func (n *Node) Close() {
	if !n.state.CompareAndSwap(int32(cluster.Work), int32(cluster.Hang)) {
		if !n.state.CompareAndSwap(int32(cluster.Busy), int32(cluster.Hang)) {
			return
		}
	}

	n.refreshServiceInstances(cluster.Hang)

	err1 := n.tasker.done()
	err2 := n.router.done()
	err3 := n.trigger.done()

	if err1 == nil {
		n.tasker.wait()
	}

	if err2 == nil {
		n.router.wait()
	}

	if err3 == nil {
		n.trigger.wait()
	}

	n.wg.Wait()

	n.runHookFunc(cluster.Close)
}

// Destroy 销毁节点服务器
// 将状态置为关闭，解注册服务实例、停止连接与传输服务器并释放内部组件资源
func (n *Node) Destroy() {
	if !n.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
		return
	}

	n.deregisterServiceInstances()

	n.stopLinkerServer()

	n.stopTransportServer()

	n.tasker.close()

	n.router.close()

	n.trigger.close()

	n.cancel()

	n.runHookFunc(cluster.Destroy)
}

// Proxy 获取节点代理
// @return @1 *Proxy 节点代理
func (n *Node) Proxy() *Proxy {
	return n.proxy
}

// 分发处理消息
// 循环监听任务器、路由器与事件触发器接收到的消息并逐一处理，队列关闭时退出
func (n *Node) dispatch() {
	n.dispatchGoid.Store(goid.Get())

	for {
		select {
		case handle, ok := <-n.tasker.receive():
			if !ok {
				return
			}

			n.tasker.handle(handle)
		case req, ok := <-n.router.receive():
			if !ok {
				return
			}

			n.router.handle(req)
		case evt, ok := <-n.trigger.receive():
			if !ok {
				return
			}

			n.trigger.handle(evt)
		}
	}
}

// 启动连接服务器
// 创建内部连接服务器用于接收网关下发的事件与消息，并以协程方式启动服务
func (n *Node) startLinkerServer() {
	linker, err := node.NewServer(&provider{node: n}, &node.ServerOptions{
		Addr:   n.opts.addr,
		Expose: n.opts.expose,
	})
	if err != nil {
		log.Fatalf("linker server create failed: %v", err)
	}

	if err = linker.Start(); err != nil {
		log.Fatalf("linker server start failed: %v", err)
	}

	n.linker = linker
}

// 停止连接服务器
func (n *Node) stopLinkerServer() {
	if err := n.linker.Stop(); err != nil {
		log.Errorf("linker server stop failed: %v", err)
	}
}

// 启动传输服务器
// 设置默认服务发现并注册服务提供者，无服务提供者时直接返回
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
// 收集路由与事件信息生成节点/Mesh服务实例并注册到注册中心
func (n *Node) registerServiceInstances() {
	routes := make([]registry.Route, 0, len(n.router.routes))
	events := make([]int, 0, len(n.trigger.events))

	for _, entity := range n.router.routes {
		routes = append(routes, registry.Route{
			ID:         entity.route,
			Internal:   entity.options.Internal,
			Stateful:   entity.options.Stateful,
			Authorized: entity.options.Authorized,
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
		Metadata: n.opts.metadata,
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
			Metadata: n.opts.metadata,
		})
	}

	if err := n.doRegisterServiceInstances(); err != nil {
		log.Fatalf("register cluster instances failed: %v", err)
	}
}

// 刷新服务实例状态
// @param state ...cluster.State 待设置的服务实例状态；缺省时仅重新注册不更新状态
func (n *Node) refreshServiceInstances(state ...cluster.State) {
	if err := n.doRefreshServiceInstances(state...); err != nil {
		log.Errorf("refresh cluster instances failed: %v", err)
	}
}

// 解注册服务实例
// 并行地对所有已注册实例执行解注册操作，并等待全部完成
func (n *Node) deregisterServiceInstances() {
	eg, ctx := errgroup.WithContext(n.ctx)
	for i := range n.instances {
		instance := n.instances[i]
		eg.Go(func() error {
			tctx, tcancel := context.WithTimeout(ctx, 3*time.Second)
			defer tcancel()
			return n.opts.registry.Deregister(tctx, instance)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Errorf("deregister cluster instances failed: %v", err)
	}
}

// 执行注册操作
// 并行地将所有服务实例注册到注册中心，等待全部注册完成
// @return @1 error 存在实例注册失败时返回的错误
func (n *Node) doRegisterServiceInstances() error {
	eg, ctx := errgroup.WithContext(n.ctx)

	for i := range n.instances {
		instance := n.instances[i]
		eg.Go(func() error {
			tctx, tcancel := context.WithTimeout(ctx, 3*time.Second)
			err := n.opts.registry.Register(tctx, instance)
			tcancel()

			return err
		})
	}

	return eg.Wait()
}

// 执行刷新实例状态操作
// @param state ...cluster.State 待设置的服务实例状态；缺省时仅重新注册不更新状态
// @return @1 error 注册失败时返回的错误
func (n *Node) doRefreshServiceInstances(state ...cluster.State) error {
	if len(state) > 0 {
		for _, instance := range n.instances {
			instance.State = state[0].String()
		}
	}

	return n.doRegisterServiceInstances()
}

// 获取状态
// @return @1 cluster.State 当前节点状态
func (n *Node) getState() cluster.State {
	return cluster.State(n.state.Load())
}

// 更新状态（仅能在Work或Busy状态间切换）
// 更新成功后会同步刷新服务实例状态到注册中心
// @param state cluster.State 目标状态，仅支持Work或Busy
// @return @1 error 状态非法、切换失败或刷新实例失败时返回的错误
func (n *Node) setState(state cluster.State) error {
	if state > cluster.Busy {
		return errors.ErrIllegalOperation
	}

	switch curr := n.getState(); curr {
	case cluster.Work, cluster.Busy:
		if curr == state {
			return nil
		}

		if n.state.CompareAndSwap(int32(curr), int32(state)) {
			return n.doRefreshServiceInstances(state)
		} else {
			return errors.ErrIllegalOperation
		}
	default:
		return errors.ErrIllegalOperation
	}
}

// 是否已关闭
// @return @1 bool 节点是否处于关闭状态
func (n *Node) isShut() bool {
	return n.getState() == cluster.Shut
}

// 执行钩子函数
// 触发指定钩子对应的全部监听器，并等待所有监听器执行完成
// @param hook cluster.Hook 钩子类型
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
// @param hook cluster.Hook 钩子类型
// @param handler HookHandler 钩子处理函数
func (n *Node) addHookListener(hook cluster.Hook, handler HookHandler) {
	switch hook {
	case cluster.Destroy:
		n.rw.Lock()
		n.hooks[hook] = append(n.hooks[hook], handler)
		n.rw.Unlock()
	default:
		if n.getState() == cluster.Shut {
			n.rw.Lock()
			n.hooks[hook] = append(n.hooks[hook], handler)
			n.rw.Unlock()
		} else {
			log.Warnf("server is working, can't add hook handler")
		}
	}
}

// 添加服务提供者
// @param name string 服务名称
// @param desc any 服务描述对象
// @param provider any 服务提供者
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
// 输出节点ID、名称、连接地址、编解码器、定位器、注册器等基础信息
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

// 完成一次等待计数
// 节点已关闭时无操作，否则执行等待组Done，用于跟踪后台任务的执行状态
func (n *Node) doDoneWait() {
	if n == nil || n.getState() == cluster.Shut {
		return
	}

	n.wg.Done()
}

// 增加一次等待计数
// 节点已关闭时无操作，否则执行等待组Add，用于跟踪后台任务的执行状态
func (n *Node) doAddWait() {
	if n == nil || n.getState() == cluster.Shut {
		return
	}

	n.wg.Add(1)
}
