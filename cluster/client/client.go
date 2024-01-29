package client

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"sync"
	"sync/atomic"
)

type HookHandler func(proxy *Proxy)

type RouteHandler func(ctx *Context)

type EventHandler func(conn *Conn)

type Client struct {
	component.Base
	opts                *options
	ctx                 context.Context
	cancel              context.CancelFunc
	routes              map[int32]RouteHandler
	events              map[cluster.Event]EventHandler
	hooks               map[cluster.Hook]HookHandler
	defaultRouteHandler RouteHandler
	proxy               *Proxy
	state               int32
	conns               sync.Map
}

func NewClient(opts ...Option) *Client {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &Client{}
	c.opts = o
	c.proxy = newProxy(c)
	c.routes = make(map[int32]RouteHandler)
	c.events = make(map[cluster.Event]EventHandler)
	c.hooks = make(map[cluster.Hook]HookHandler)
	c.ctx, c.cancel = context.WithCancel(o.ctx)

	c.setState(cluster.Shut)

	return c
}

// Name 组件名称
func (c *Client) Name() string {
	return c.opts.name
}

// Init 初始化节点
func (c *Client) Init() {
	if c.opts.client == nil {
		log.Fatal("client plugin is not injected")
	}

	if c.opts.codec == nil {
		log.Fatal("codec plugin is not injected")
	}

	c.runHookFunc(cluster.Init)
}

// Start 启动组件
func (c *Client) Start() {
	c.setState(cluster.Work)

	c.opts.client.OnConnect(c.handleConnect)
	c.opts.client.OnDisconnect(c.handleDisconnect)
	c.opts.client.OnReceive(c.handleReceive)

	c.runHookFunc(cluster.Start)
}

// Destroy 销毁组件
func (c *Client) Destroy() {
	c.setState(cluster.Shut)

	c.runHookFunc(cluster.Destroy)
}

// Proxy 获取节点代理
func (c *Client) Proxy() *Proxy {
	return c.proxy
}

// 处理连接打开
func (c *Client) handleConnect(conn network.Conn) {
	handler, ok := c.events[cluster.Connect]
	if !ok {
		return
	}

	cc := &Conn{conn: conn, client: c}

	c.conns.Store(conn, cc)

	handler(cc)
}

// 处理断开连接
func (c *Client) handleDisconnect(conn network.Conn) {
	handler, ok := c.events[cluster.Disconnect]
	if !ok {
		return
	}

	val, ok := c.conns.Load(conn)
	if !ok {
		return
	}

	handler(val.(*Conn))

	c.conns.Delete(conn)
}

// 处理接收到的消息
func (c *Client) handleReceive(conn network.Conn, data []byte) {
	val, ok := c.conns.Load(conn)
	if !ok {
		return
	}

	message, err := packet.UnpackMessage(data)
	if err != nil {
		log.Errorf("unpack message failed: %v", err)
		return
	}

	handler, ok := c.routes[message.Route]
	if ok {
		handler(&Context{
			ctx:     context.Background(),
			conn:    val.(*Conn),
			message: message,
		})
	} else if c.defaultRouteHandler != nil {
		c.defaultRouteHandler(&Context{
			ctx:     context.Background(),
			conn:    val.(*Conn),
			message: message,
		})
	} else {
		log.Errorf("route handler is not registered, route:%v", message.Route)
	}
}

// 拨号
func (c *Client) dial(addr ...string) (*Conn, error) {
	if c.getState() == cluster.Shut {
		return nil, errors.ErrClientShut
	}

	conn, err := c.opts.client.Dial(addr...)
	if err != nil {
		return nil, err
	}

	return &Conn{conn: conn, client: c}, nil
}

// 添加路由处理器
func (c *Client) addRouteHandler(route int32, handler RouteHandler) {
	if c.getState() == cluster.Shut {
		c.routes[route] = handler
	} else {
		log.Warnf("client is working, can't add route handler")
	}
}

// 默认路由处理器
func (c *Client) setDefaultRouteHandler(handler RouteHandler) {
	if c.getState() == cluster.Shut {
		c.defaultRouteHandler = handler
	} else {
		log.Warnf("client is working, can't set default route handler")
	}
}

// 添加事件处理器
func (c *Client) addEventListener(event cluster.Event, handler EventHandler) {
	if c.getState() == cluster.Shut {
		c.events[event] = handler
	} else {
		log.Warnf("client is working, can't add event handler")
	}
}

// 添加钩子监听器
func (c *Client) addHookListener(hook cluster.Hook, handler HookHandler) {
	if c.getState() == cluster.Shut {
		c.hooks[hook] = handler
	} else {
		log.Warnf("client is working, can't add hook handler")
	}
}

// 设置状态
func (c *Client) setState(state cluster.State) {
	atomic.StoreInt32(&c.state, int32(state))
}

// 获取状态
func (c *Client) getState() cluster.State {
	return cluster.State(atomic.LoadInt32(&c.state))
}

// 执行钩子函数
func (c *Client) runHookFunc(hook cluster.Hook) {
	if handler, ok := c.hooks[hook]; ok {
		handler(c.proxy)
	}
}
