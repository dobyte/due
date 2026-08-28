package mesh

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
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xcall"
)

// HookHandler 微服务钩子处理函数
type HookHandler func(proxy *Proxy)

// Mesh 微服务服务器
type Mesh struct {
	component.Base
	opts        *options
	ctx         context.Context
	cancel      context.CancelFunc
	state       atomic.Int32
	proxy       *Proxy
	transporter transport.Server
	services    []*serviceEntity
	instance    *registry.ServiceInstance
	rw          sync.RWMutex
	hooks       map[cluster.Hook][]HookHandler
}

// 服务实体
type serviceEntity struct {
	name     string // 服务名称;用于定位服务发现
	desc     any    // 服务描述(grpc为desc描述对象; rpcx为服务路径)
	provider any    // 服务提供者
}

// NewMesh 创建微服务服务器
// 创建后会初始化代理与内部组件，并处于关闭状态
// @param opts ...Option 微服务配置项
// @return @1 *Mesh 微服务服务器实例
func NewMesh(opts ...Option) *Mesh {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	m := &Mesh{}
	m.opts = o
	m.hooks = make(map[cluster.Hook][]HookHandler)
	m.services = make([]*serviceEntity, 0)
	m.ctx, m.cancel = context.WithCancel(o.ctx)
	m.proxy = newProxy(m)
	m.state.Store(int32(cluster.Shut))

	return m
}

// Name 组件名称
// @return @1 string 组件名称
func (m *Mesh) Name() string {
	return m.opts.name
}

// Init 初始化微服务
// 校验编解码器、注册器与传输器等必要配置，缺失时直接终止进程
func (m *Mesh) Init() {
	if m.opts.codec == nil {
		log.Fatal("codec component is not injected")
	}

	if m.opts.registry == nil {
		log.Fatal("registry component is not injected")
	}

	if m.opts.transporter == nil {
		log.Fatal("transporter component is not injected")
	}

	m.runHookFunc(cluster.Init)
}

// Start 启动微服务
// 将状态置为工作中，随后启动传输服务器、注册服务实例并开启监听
func (m *Mesh) Start() {
	if !m.state.CompareAndSwap(int32(cluster.Shut), int32(cluster.Work)) {
		return
	}

	m.startTransportServer()

	m.registerServiceInstance()

	m.proxy.watch()

	m.printInfo()

	m.runHookFunc(cluster.Start)
}

// Close 关闭微服务
// 将状态置为挂起，刷新服务实例状态到注册中心
func (m *Mesh) Close() {
	if !m.state.CompareAndSwap(int32(cluster.Work), int32(cluster.Hang)) {
		if !m.state.CompareAndSwap(int32(cluster.Busy), int32(cluster.Hang)) {
			return
		}
	}

	m.refreshServiceInstance()

	m.runHookFunc(cluster.Close)
}

// Destroy 销毁微服务服务器
// 将状态置为关闭，解注册服务实例、停止传输服务器并释放内部组件资源
func (m *Mesh) Destroy() {
	if !m.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
		return
	}

	m.deregisterServiceInstance()

	m.stopTransportServer()

	m.cancel()

	m.runHookFunc(cluster.Destroy)
}

// Proxy 获取微服务代理
// @return @1 *Proxy 微服务代理
func (m *Mesh) Proxy() *Proxy {
	return m.proxy
}

// 启动传输服务器
// 设置默认服务发现并注册服务提供者，无服务提供者时直接终止进程
func (m *Mesh) startTransportServer() {
	if len(m.services) == 0 {
		log.Fatal("no service registered")
	}

	m.opts.transporter.SetDefaultDiscovery(m.opts.registry)

	transporter, err := m.opts.transporter.NewServer()
	if err != nil {
		log.Fatalf("transport server create failed: %v", err)
	}

	m.transporter = transporter

	for _, entity := range m.services {
		if err = m.transporter.RegisterService(entity.desc, entity.provider); err != nil {
			log.Fatalf("register service failed: %v", err)
		}
	}

	go func() {
		if err = m.transporter.Start(); err != nil {
			log.Fatalf("transport server start failed: %v", err)
		}
	}()
}

// 停止传输服务器
func (m *Mesh) stopTransportServer() {
	if m.transporter == nil {
		return
	}

	if err := m.transporter.Stop(); err != nil {
		log.Errorf("transport server stop failed: %v", err)
	}
}

// 注册服务实例
// 生成微服务服务实例并注册到注册中心
func (m *Mesh) registerServiceInstance() {
	m.instance = &registry.ServiceInstance{
		ID:       m.opts.id,
		Name:     cluster.Mesh.String(),
		Kind:     cluster.Mesh.String(),
		Alias:    m.opts.name,
		State:    m.getState().String(),
		Endpoint: m.transporter.Endpoint().String(),
		Services: make([]string, 0, len(m.services)),
		Weight:   m.opts.weight,
		Metadata: m.opts.metadata,
	}

	for _, item := range m.services {
		m.instance.Services = append(m.instance.Services, item.name)
	}

	if err := m.doRegisterServiceInstance(); err != nil {
		log.Fatalf("register cluster instance failed: %v", err)
	}
}

// 刷新服务实例状态
// 以当前状态重新刷新注册中心中的服务实例
func (m *Mesh) refreshServiceInstance() {
	if err := m.doRefreshServiceInstance(m.getState()); err != nil {
		log.Errorf("refresh cluster instance failed: %v", err)
	}
}

// 解注册服务实例
func (m *Mesh) deregisterServiceInstance() {
	ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
	err := m.opts.registry.Deregister(ctx, m.instance)
	cancel()
	if err != nil {
		log.Errorf("deregister cluster instance failed: %v", err)
	}
}

// 执行注册操作
// @return @1 error 注册失败时返回的错误
func (m *Mesh) doRegisterServiceInstance() error {
	ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
	err := m.opts.registry.Register(ctx, m.instance)
	cancel()

	return err
}

// 执行刷新实例状态操作
// @param state ...cluster.State 待设置的服务实例状态；缺省时仅重新注册不更新状态
// @return @1 error 注册失败时返回的错误
func (m *Mesh) doRefreshServiceInstance(state ...cluster.State) error {
	if len(state) > 0 {
		m.instance.State = state[0].String()
	}

	return m.doRegisterServiceInstance()
}

// 执行钩子函数
// 触发指定钩子对应的全部监听器，并等待所有监听器执行完成
// @param hook cluster.Hook 钩子类型
func (m *Mesh) runHookFunc(hook cluster.Hook) {
	m.rw.RLock()

	if handlers, ok := m.hooks[hook]; ok {
		wg := &sync.WaitGroup{}
		wg.Add(len(handlers))

		for i := range handlers {
			handler := handlers[i]
			xcall.Go(func() {
				handler(m.proxy)
				wg.Done()
			})
		}

		m.rw.RUnlock()

		wg.Wait()
	} else {
		m.rw.RUnlock()
	}
}

// 添加钩子监听器
// @param hook cluster.Hook 钩子类型
// @param handler HookHandler 钩子处理函数
func (m *Mesh) addHookListener(hook cluster.Hook, handler HookHandler) {
	switch hook {
	case cluster.Destroy:
		m.rw.Lock()
		m.hooks[hook] = append(m.hooks[hook], handler)
		m.rw.Unlock()
	default:
		if m.getState() == cluster.Shut {
			m.rw.Lock()
			m.hooks[hook] = append(m.hooks[hook], handler)
			m.rw.Unlock()
		} else {
			log.Warnf("server is working, can't add hook handler")
		}
	}
}

// 添加服务提供者
// @param name string 服务名称
// @param desc any 服务描述对象
// @param provider any 服务提供者
func (m *Mesh) addServiceProvider(name string, desc, provider any) {
	if m.getState() == cluster.Shut {
		m.services = append(m.services, &serviceEntity{
			name:     name,
			desc:     desc,
			provider: provider,
		})
	} else {
		log.Warnf("mesh server is working, can't add service provider")
	}
}

// 获取状态
// @return @1 cluster.State 当前微服务状态
func (m *Mesh) getState() cluster.State {
	return cluster.State(m.state.Load())
}

// 更新状态（仅能在Work或Busy状态间切换）
// 更新成功后会同步刷新服务实例状态到注册中心
// @param state cluster.State 目标状态，仅支持Work或Busy
// @return @1 error 状态非法、切换失败或刷新实例失败时返回的错误
func (m *Mesh) setState(state cluster.State) error {
	if state > cluster.Busy {
		return errors.ErrIllegalOperation
	}

	switch curr := m.getState(); curr {
	case cluster.Work, cluster.Busy:
		if curr == state {
			return nil
		}

		if m.state.CompareAndSwap(int32(curr), int32(state)) {
			return m.doRefreshServiceInstance(state)
		} else {
			return errors.ErrIllegalOperation
		}
	default:
		return errors.ErrIllegalOperation
	}
}

// 是否已关闭
// @return @1 bool 微服务是否处于关闭状态
func (m *Mesh) isShut() bool {
	return m.getState() == cluster.Shut
}

// 打印组件信息
// 输出微服务ID、名称、编解码器、定位器、注册器等基础信息
func (m *Mesh) printInfo() {
	infos := make([]string, 0, 7)
	infos = append(infos, fmt.Sprintf("ID: %s", m.opts.id))
	infos = append(infos, fmt.Sprintf("Name: %s", m.Name()))
	infos = append(infos, fmt.Sprintf("Codec: %s", m.opts.codec.Name()))

	if m.opts.locator != nil {
		infos = append(infos, fmt.Sprintf("Locator: %s", m.opts.locator.Name()))
	} else {
		infos = append(infos, "Locator: -")
	}

	infos = append(infos, fmt.Sprintf("Registry: %s", m.opts.registry.Name()))

	if m.opts.encryptor != nil {
		infos = append(infos, fmt.Sprintf("Encryptor: %s", m.opts.encryptor.Name()))
	} else {
		infos = append(infos, "Encryptor: -")
	}

	infos = append(infos, fmt.Sprintf("Transporter: %s", m.opts.transporter.Name()))

	info.PrintBoxInfo("Mesh", infos...)
}
