package client

import (
	"github.com/dobyte/due/v2/cluster"
)

type Proxy struct {
	client *Client // 客户端
}

func newProxy(client *Client) *Proxy {
	return &Proxy{client: client}
}

// ID 获取客户端ID
func (p *Proxy) ID() string {
	return p.client.opts.id
}

// Name 获取客户端名称
func (p *Proxy) Name() string {
	return p.client.opts.name
}

// AddRouteHandler 添加路由处理器
func (p *Proxy) AddRouteHandler(route int32, handler RouteHandler) {
	p.client.addRouteHandler(route, handler)
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
func (p *Proxy) SetDefaultRouteHandler(handler RouteHandler) {
	p.client.setDefaultRouteHandler(handler)
}

// AddEventListener 添加事件监听器
func (p *Proxy) AddEventListener(event cluster.Event, handler EventHandler) {
	p.client.addEventListener(event, handler)
}

// AddHookListener 添加钩子监听器
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.client.addHookListener(hook, handler)
}

// Dial 拨号
func (p *Proxy) Dial(opts ...DialOption) (*Conn, error) {
	return p.client.dial(opts...)
}
