package client

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/network"
)

// Proxy 客户端代理
// 对外暴露路由、事件、钩子的注册与拨号等能力
type Proxy struct {
	client *Client // 客户端
}

// newProxy 创建客户端代理
// @param client *Client 客户端
// @return @1 *Proxy 客户端代理
func newProxy(client *Client) *Proxy {
	return &Proxy{client: client}
}

// ID 获取客户端ID
// @return @1 string 客户端ID
func (p *Proxy) ID() string {
	return p.client.opts.id
}

// Name 获取客户端名称
// @return @1 string 客户端名称
func (p *Proxy) Name() string {
	return p.client.opts.name
}

// AddRouteHandler 添加路由处理器
// @param route int32 路由号
// @param handler RouteHandler 路由处理函数
func (p *Proxy) AddRouteHandler(route int32, handler RouteHandler) {
	p.client.addRouteHandler(route, handler)
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
// @param handler RouteHandler 默认路由处理函数
func (p *Proxy) SetDefaultRouteHandler(handler RouteHandler) {
	p.client.setDefaultRouteHandler(handler)
}

// AddEventListener 添加事件监听器
// @param event cluster.Event 事件类型
// @param handler EventHandler 事件处理函数
func (p *Proxy) AddEventListener(event cluster.Event, handler EventHandler) {
	p.client.addEventListener(event, handler)
}

// AddHookListener 添加钩子监听器
// @param hook cluster.Hook 钩子类型
// @param handler HookHandler 钩子处理函数
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.client.addHookListener(hook, handler)
}

// Dial 拨号
// 建立与服务端的连接，返回封装后的连接对象
// @param opts ...DialOption 拨号配置项
// @return @1 *Conn 连接对象
// @return @2 error 错误信息
func (p *Proxy) Dial(opts ...DialOption) (*Conn, error) {
	return p.client.dial(opts...)
}

// Client 获取网络客户端
// @return @1 network.Client 网络客户端
func (p *Proxy) Client() network.Client {
	return p.client.opts.client
}
