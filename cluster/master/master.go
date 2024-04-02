package master

import (
	"context"
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/component"
	"github.com/symsimmy/due/config"
	_ "github.com/symsimmy/due/crypto/ecc"
	_ "github.com/symsimmy/due/crypto/rsa"
	_ "github.com/symsimmy/due/encoding/json"
	_ "github.com/symsimmy/due/encoding/proto"
	_ "github.com/symsimmy/due/encoding/xml"
	"github.com/symsimmy/due/common/endpoint"
	xnet "github.com/symsimmy/due/common/net"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/registry"
	"sync/atomic"
	"time"
	"unsafe"
)

type Master struct {
	component.Base
	ctx      context.Context
	cancel   context.CancelFunc
	opts     *options
	proxy    *Proxy
	state    cluster.State
	instance *registry.ServiceInstance
	endpoint *endpoint.Endpoint
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

	m.proxy.watch(m.ctx)

	m.startCatServer()

	m.registerServiceInstance()

	m.debugPrint()
}

// Destroy 销毁组件
func (m *Master) Destroy() {
	m.stopCatServer()

	m.cancel()
}

// Proxy 获取管理服代理
func (m *Master) Proxy() *Proxy {
	return m.proxy
}

func (m *Master) startCatServer() {
	if m.opts.catServer != nil {
		m.opts.catServer.Start()
	}
}

func (m *Master) stopCatServer() {
	if m.opts.catServer != nil {
		m.opts.catServer.Destroy()
	}
}

func (m *Master) debugPrint() {
	log.Debugf("master server startup successful")
}

func (m *Master) registerServiceInstance() {
	addr := config.Get("config.http.addr", ":8080").String()
	_, exposeAddr, err := xnet.ParseAddr(addr)
	if err != nil {
		log.Errorf("parse addr[%+v] failed.err:%+v", addr, err)
	}

	m.endpoint = endpoint.NewEndpoint("http", exposeAddr, false)

	m.instance = &registry.ServiceInstance{
		ID:       m.opts.id,
		Name:     string(cluster.Master),
		Kind:     cluster.Node,
		Alias:    m.opts.name,
		State:    m.getState(),
		Endpoint: m.endpoint.String(),
	}

	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	err = m.opts.registry.Register(ctx, m.instance)
	cancel()
	if err != nil {
		log.Fatalf("register master instance failed: %v", err)
	}
}

// 设置节点状态
func (m *Master) setState(state cluster.State) {
	if m.checkState(state) {
		return
	}

	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&m.state)), unsafe.Pointer(&state))

	if m.instance != nil {
		m.instance.State = m.getState()
		for i := 0; i < 3; i++ {
			ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
			err := m.opts.registry.Register(ctx, m.instance)
			cancel()
			if err == nil {
				break
			}
		}
	}

	return
}

// 获取节点状态
func (m *Master) getState() cluster.State {
	if state := (*cluster.State)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.state)))); state == nil {
		return cluster.Shut
	} else {
		return *state
	}
}

// 检测节点状态
func (m *Master) checkState(state cluster.State) bool {
	return m.getState() == state
}
