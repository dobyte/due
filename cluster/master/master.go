package master

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/log"
	"sync/atomic"
	"time"
)

const timeout = 5 * time.Second

type HookHandler func(proxy *Proxy)

type Master struct {
	component.Base
	ctx    context.Context
	cancel context.CancelFunc
	opts   *options
	state  int32
	proxy  *Proxy
	hooks  map[cluster.Hook]HookHandler
}

func NewMaster(opts ...Option) *Master {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	m := &Master{}
	m.opts = o
	m.proxy = newProxy(m)
	m.ctx, m.cancel = context.WithCancel(o.ctx)

	m.setState(cluster.Shut)

	return m
}

// Name 组件名称
func (m *Master) Name() string {
	return m.opts.name
}

// Init 初始化组件
func (m *Master) Init() {
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

// Start 启动组件
func (m *Master) Start() {
	m.setState(cluster.Work)

	m.opts.transporter.SetDefaultDiscovery(m.opts.registry)

	m.proxy.watch(m.ctx)

	m.debugPrint()

	m.runHookFunc(cluster.Start)
}

// Destroy 销毁组件
func (m *Master) Destroy() {
	m.cancel()

	if m.opts.configurator != nil {
		m.opts.configurator.Close()
	}

	m.runHookFunc(cluster.Destroy)
}

// Proxy 获取管理服代理
func (m *Master) Proxy() *Proxy {
	return m.proxy
}

func (m *Master) debugPrint() {
	log.Debugf("master server startup successful")
}

// 设置状态
func (m *Master) setState(state cluster.State) {
	atomic.StoreInt32(&m.state, int32(state))
}

// 获取状态
func (m *Master) getState() cluster.State {
	return cluster.State(atomic.LoadInt32(&m.state))
}

// 执行钩子函数
func (m *Master) runHookFunc(hook cluster.Hook) {
	if handler, ok := m.hooks[hook]; ok {
		handler(m.proxy)
	}
}

// 添加钩子监听器
func (m *Master) addHookListener(hook cluster.Hook, handler HookHandler) {
	if m.getState() == cluster.Shut {
		m.hooks[hook] = handler
	} else {
		log.Warnf("the master server is working, can't add hook handler")
	}
}
