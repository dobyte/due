package mesh

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/component"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/utils/xuuid"
	"golang.org/x/sync/errgroup"
	"time"
)

type Mesh struct {
	component.Base
	opts      *options
	ctx       context.Context
	cancel    context.CancelFunc
	state     cluster.State
	proxy     *Proxy
	services  []*serviceEntity
	instances []*registry.ServiceInstance
	rpc       transport.Server
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
	m.state = cluster.Shut
	m.services = make([]*serviceEntity, 0)
	m.instances = make([]*registry.ServiceInstance, 0)
	m.proxy = newProxy(m)
	m.ctx, m.cancel = context.WithCancel(o.ctx)

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
}

// Start 启动
func (m *Mesh) Start() {
	m.state = cluster.Work

	m.opts.transporter.SetDefaultDiscovery(m.opts.registry)

	m.startRPCServer()

	m.registerServiceInstances()

	m.proxy.watch(m.ctx)

	m.debugPrint()
}

// Destroy 销毁网关服务器
func (m *Mesh) Destroy() {
	m.deregisterServiceInstances()

	m.stopRPCServer()

	m.cancel()
}

// Proxy 获取节点代理
func (m *Mesh) Proxy() *Proxy {
	return m.proxy
}

// 启动RPC服务器
func (m *Mesh) startRPCServer() {
	var err error

	m.rpc, err = m.opts.transporter.NewServiceServer()
	if err != nil {
		log.Fatalf("rpc server create failed: %v", err)
	}

	for _, entity := range m.services {
		err = m.rpc.RegisterService(entity.desc, entity.provider)
		if err != nil {
			log.Fatalf("register service failed: %v", err)
		}
	}

	go func() {
		if err = m.rpc.Start(); err != nil {
			log.Fatalf("rpc server start failed: %v", err)
		}
	}()
}

// 停止RPC服务器
func (m *Mesh) stopRPCServer() {
	if err := m.rpc.Stop(); err != nil {
		log.Errorf("rpc server stop failed: %v", err)
	}
}

// 注册服务实例
func (m *Mesh) registerServiceInstances() {
	endpoint := m.rpc.Endpoint().String()

	for _, entity := range m.services {
		id, err := xuuid.UUID()
		if err != nil {
			log.Fatalf("generate service id failed: %v", err)
		}

		m.instances = append(m.instances, &registry.ServiceInstance{
			ID:       id,
			Name:     entity.name,
			Kind:     cluster.Mesh,
			Alias:    entity.name,
			State:    cluster.Work,
			Endpoint: endpoint,
		})
	}

	eg, ctx := errgroup.WithContext(m.ctx)
	for i := range m.instances {
		instance := m.instances[i]
		eg.Go(func() error {
			rctx, rcancel := context.WithTimeout(ctx, 10*time.Second)
			defer rcancel()
			return m.opts.registry.Register(rctx, instance)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Fatalf("register service instance failed: %v", err)
	}
}

// 解注册服务实例
func (m *Mesh) deregisterServiceInstances() {
	eg, ctx := errgroup.WithContext(m.ctx)
	for i := range m.instances {
		instance := m.instances[i]
		eg.Go(func() error {
			dctx, dcancel := context.WithTimeout(ctx, 10*time.Second)
			defer dcancel()
			return m.opts.registry.Deregister(dctx, instance)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Errorf("deregister service instance failed: %v", err)
	}
}

func (m *Mesh) debugPrint() {
	log.Debugf("mesh server startup successful")
	log.Debugf("%s server listen on %s", m.rpc.Scheme(), m.rpc.Addr())
}
