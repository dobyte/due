package master

import (
	"context"
	"github.com/dobyte/due/component"
	_ "github.com/dobyte/due/crypto/ecc"
	_ "github.com/dobyte/due/crypto/rsa"
	_ "github.com/dobyte/due/encoding/json"
	_ "github.com/dobyte/due/encoding/proto"
	_ "github.com/dobyte/due/encoding/xml"
	"github.com/dobyte/due/log"
)

type Master struct {
	component.Base
	ctx    context.Context
	cancel context.CancelFunc
	opts   *options
	proxy  *proxy
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
	return m.opts.id
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
	m.proxy.watch(m.ctx)

	m.debugPrint()
}

// Destroy 销毁组件
func (m *Master) Destroy() {
	m.cancel()
}

// Proxy 获取管理服代理
func (m *Master) Proxy() Proxy {
	return m.proxy
}

func (m *Master) debugPrint() {
	log.Debugf("The master server startup successful")
}
