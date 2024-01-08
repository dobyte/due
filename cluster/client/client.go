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
	rw                  sync.RWMutex
	state               cluster.State
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
	c.state = cluster.Shut
	c.ctx, c.cancel = context.WithCancel(o.ctx)

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

	if handler, ok := c.hooks[cluster.Init]; ok {
		handler(c.proxy)
	}

	c.state = cluster.Work
}

// Start 启动组件
func (c *Client) Start() {
	c.opts.client.OnConnect(c.handleConnect)
	c.opts.client.OnDisconnect(c.handleDisconnect)
	c.opts.client.OnReceive(c.handleReceive)

	if handler, ok := c.hooks[cluster.Start]; ok {
		handler(c.proxy)
	}
}

// Destroy 销毁组件
func (c *Client) Destroy() {
	if handler, ok := c.hooks[cluster.Destroy]; ok {
		handler(c.proxy)
	}

	c.rw.Lock()
	c.conn = nil
	c.state = cluster.Shut
	c.rw.Unlock()
}

// Proxy 获取节点代理
func (c *Client) Proxy() *Proxy {
	return c.proxy
}

// 处理连接打开
func (c *Client) handleConnect(conn network.Conn) {
	c.rw.Lock()
	isNew := c.conn == nil
	c.conn = conn
	c.rw.Unlock()

	var (
		ok      bool
		handler EventHandler
	)

	if !isNew {
		handler, ok = c.events[cluster.Reconnect]
	}

	if !ok {
		handler, ok = c.events[cluster.Connect]
	}

	if !ok {
		return
	}

	handler(c.proxy)
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
func (c *Client) handleReceive(_ network.Conn, data []byte) {
	message, err := packet.Unpack(data)
	if err != nil {
		log.Errorf("unpack message failed: %v", err)
		return
	}

	handler, ok := c.routes[message.Route]
	if ok {
		handler(&Context{
			ctx:     context.Background(),
			client:  c,
			message: message,
		})
	} else if c.defaultRouteHandler != nil {
		c.defaultRouteHandler(&Context{
			ctx:     context.Background(),
			client:  c,
			message: message,
		})
	} else {
		log.Errorf("route handler is not registered, route:%v", message.Route)
	}
}

// 拨号
func (c *Client) dial(addr ...string) (*Conn, error) {
	c.rw.RLock()
	isShut := c.state == cluster.Shut
	c.rw.RUnlock()

	if isShut {
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
	if c.state == cluster.Shut {
		c.routes[route] = handler
	} else {
		log.Warnf("client is working, can't add route handler")
	}
}

// 默认路由处理器
func (c *Client) setDefaultRouteHandler(handler RouteHandler) {
	if c.state == cluster.Shut {
		c.defaultRouteHandler = handler
	} else {
		log.Warnf("client is working, can't set default route handler")
	}
}

// 添加事件处理器
func (c *Client) addEventListener(event cluster.Event, handler EventHandler) {
	if c.state == cluster.Shut {
		c.events[event] = handler
	} else {
		log.Warnf("client is working, can't add event handler")
	}
}

// 添加钩子监听器
func (c *Client) addHookListener(hook cluster.Hook, handler HookHandler) {
	if c.state == cluster.Shut {
		c.hooks[hook] = handler
	} else {
		log.Warnf("client is working, can't add hook handler")
	}
}
