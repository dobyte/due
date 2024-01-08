package mesh

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xuuid"
	"golang.org/x/sync/errgroup"
	"sync/atomic"
	"time"
)

const timeout = 5 * time.Second

type HookHandler func(proxy *Proxy)

type Mesh struct {
	component.Base
	opts        *options
	ctx         context.Context
	cancel      context.CancelFunc
	state       int32
	proxy       *Proxy
	services    []*serviceEntity
	instances   []*registry.ServiceInstance
	hooks       map[cluster.Hook]HookHandler
	transporter transport.Server
}

type serviceEntity struct {
	name     string      // 服务名称;用于定位服务发现
	desc     interface{} // 服务描述(grpc为desc描述对象; rpcx为服务路径)
	provider interface{} // 服务提供者
}

func NewMesh(opts ...Option) *Mesh {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	m := &Mesh{}
	m.opts = o
	m.services = make([]*serviceEntity, 0)
	m.instances = make([]*registry.ServiceInstance, 0)
	m.proxy = newProxy(m)
	m.ctx, m.cancel = context.WithCancel(o.ctx)

	m.setState(cluster.Shut)

	return m
}

// Name 组件名称
func (m *Mesh) Name() string {
	return m.opts.name
}

// Init 初始化节点
func (m *Mesh) Init() {
	if m.opts.codec == nil {
		log.Fatal("codec component is not injected")
	}

	if m.opts.locator == nil {
		log.Fatal("locator component is not injected")
	}

	if m.opts.registry == nil {
		log.Fatal("registry component is not injected")
	}

	if m.opts.transporter == nil {
		log.Fatal("transporter component is not injected")
	}

	m.runHookFunc(cluster.Init)
}

// Start 启动
func (m *Mesh) Start() {
	m.setState(cluster.Work)

	m.startTransporter()

	m.registerServiceInstances()

	m.proxy.watch(m.ctx)

	m.debugPrint()

	m.runHookFunc(cluster.Start)
}

// Destroy 销毁网关服务器
func (m *Mesh) Destroy() {
	m.setState(cluster.Shut)

	m.deregisterServiceInstances()

	m.stopTransporter()

	m.cancel()

	m.runHookFunc(cluster.Destroy)
}

// Proxy 获取节点代理
func (m *Mesh) Proxy() *Proxy {
	return m.proxy
}

// 启动传输服务器
func (m *Mesh) startTransporter() {
	m.opts.transporter.SetDefaultDiscovery(m.opts.registry)

	transporter, err := m.opts.transporter.NewServiceServer()
	if err != nil {
		log.Fatalf("transporter create failed: %v", err)
	}

	m.transporter = transporter

	for _, entity := range m.services {
		err = m.transporter.RegisterService(entity.desc, entity.provider)
		if err != nil {
			log.Fatalf("register service failed: %v", err)
		}
	}

	go func() {
		if err = m.transporter.Start(); err != nil {
			log.Fatalf("transporter start failed: %v", err)
		}
	}()
}

// 停止传输服务器
func (m *Mesh) stopTransporter() {
	if err := m.transporter.Stop(); err != nil {
		log.Errorf("transporter stop failed: %v", err)
	}
}

// 注册服务实例
func (m *Mesh) registerServiceInstances() {
	var (
		id       string
		check    = make(map[string]struct{}, len(m.services))
		endpoint = m.transporter.Endpoint().String()
		state    = m.getState().String()
	)

	for _, entity := range m.services {
		for {
			id = xuuid.UUID()
			if _, ok := check[id]; !ok {
				check[id] = struct{}{}
				break
			}
		}

		m.instances = append(m.instances, &registry.ServiceInstance{
			ID:       id,
			Name:     entity.name,
			Kind:     cluster.Mesh.String(),
			Alias:    entity.name,
			State:    state,
			Endpoint: endpoint,
		})
	}

	eg, ctx := errgroup.WithContext(m.ctx)
	for i := range m.instances {
		instance := m.instances[i]
		eg.Go(func() error {
			rctx, rcancel := context.WithTimeout(ctx, timeout)
			defer rcancel()
			return m.opts.registry.Register(rctx, instance)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Fatalf("register mesh instance failed: %v", err)
	}
}

// 解注册服务实例
func (m *Mesh) deregisterServiceInstances() {
	eg, ctx := errgroup.WithContext(m.ctx)
	for i := range m.instances {
		instance := m.instances[i]
		eg.Go(func() error {
			dctx, dcancel := context.WithTimeout(ctx, timeout)
			defer dcancel()
			return m.opts.registry.Deregister(dctx, instance)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Errorf("deregister mesh instance failed: %v", err)
	}
}

// 设置状态
func (m *Mesh) setState(state cluster.State) {
	atomic.StoreInt32(&m.state, int32(state))
}

// 获取状态
func (m *Mesh) getState() cluster.State {
	return cluster.State(atomic.LoadInt32(&m.state))
}

func (m *Mesh) debugPrint() {
	log.Debugf("mesh server startup successful")
	log.Debugf("%s server listen on %s", m.transporter.Scheme(), m.transporter.Addr())
}

// 执行钩子函数
func (m *Mesh) runHookFunc(hook cluster.Hook) {
	if handler, ok := m.hooks[hook]; ok {
		handler(m.proxy)
	}
}

// 添加钩子监听器
func (m *Mesh) addHookListener(hook cluster.Hook, handler HookHandler) {
	if m.getState() == cluster.Shut {
		m.hooks[hook] = handler
	} else {
		log.Warnf("the mesh server is working, can't add hook handler")
	}
}

// 添加服务提供者
func (m *Mesh) addServiceProvider(name string, desc, provider any) {
	if m.getState() == cluster.Shut {
		m.services = append(m.services, &serviceEntity{
			name:     name,
			desc:     desc,
			provider: provider,
		})
	} else {
		log.Warnf("the mesh server is working, can't add service provider")
	}
}
