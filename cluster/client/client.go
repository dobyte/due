package client

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/component"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/network"
	"github.com/dobyte/due/packet"
	"sync"
)

type RouteHandler func(req Request)

type EventHandler func()

type Client struct {
	component.Base
	opts                *options
	ctx                 context.Context
	cancel              context.CancelFunc
	routes              map[int32]RouteHandler
	events              map[cluster.Event]EventHandler
	defaultRouteHandler RouteHandler
	proxy               *proxy
	rw                  sync.RWMutex
	state               cluster.State
	conn                network.Conn
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

	c.rw.Lock()
	c.state = cluster.Work
	c.rw.Unlock()
}

// Start 启动组件
func (c *Client) Start() {
	c.opts.client.OnConnect(c.handleConnect)
	c.opts.client.OnDisconnect(c.handleDisconnect)
	c.opts.client.OnReceive(c.handleReceive)

	_, err := c.opts.client.Dial()
	if err != nil {
		log.Fatalf("connect server failed: %v", err)
	}
}

// Destroy 销毁组件
func (c *Client) Destroy() {
	c.rw.Lock()
	c.conn = nil
	c.state = cluster.Shut
	c.rw.Unlock()
}

// Proxy 获取节点代理
func (c *Client) Proxy() Proxy {
	return c.proxy
}

// 处理连接打开
func (c *Client) handleConnect(conn network.Conn) {
	c.rw.Lock()
	defer c.rw.Unlock()

	var (
		ok      bool
		handler EventHandler
	)

	if c.conn == nil {
		handler, ok = c.events[cluster.Connect]
	} else {
		handler, ok = c.events[cluster.Reconnect]
		if !ok {
			handler, ok = c.events[cluster.Connect]
		}
	}

	c.conn = conn

	if !ok {
		return
	}

	handler()
}

// 处理断开连接
func (c *Client) handleDisconnect(_ network.Conn) {
	handler, ok := c.events[cluster.Disconnect]
	if !ok {
		return
	}

	handler()
}

// 处理接收到的消息
func (c *Client) handleReceive(_ network.Conn, data []byte, _ int) {
	message, err := packet.Unpack(data)
	if err != nil {
		log.Errorf("unpack message failed: %v", err)
		return
	}

	handler, ok := c.routes[message.Route]
	if ok {
		handler(&request{client: c, message: message})
	} else if c.defaultRouteHandler != nil {
		c.defaultRouteHandler(&request{client: c, message: message})
	} else {
		log.Errorf("the route handler is not registered, route:%v", message.Route)
	}
}

// 添加路由处理器
func (c *Client) addRouteHandler(route int32, handler RouteHandler) {
	c.rw.RLock()
	state := c.state
	c.rw.RUnlock()

	if state == cluster.Shut {
		c.routes[route] = handler
	} else {
		log.Warnf("the client is working, can't add route handler")
	}
}

// 默认路由处理器
func (c *Client) setDefaultRouteHandler(handler RouteHandler) {
	c.rw.RLock()
	state := c.state
	c.rw.RUnlock()

	if state == cluster.Shut {
		c.defaultRouteHandler = handler
	} else {
		log.Warnf("the client is working, can't set default route handler")
	}
}

// 添加事件处理器
func (c *Client) addEventListener(event cluster.Event, handler EventHandler) {
	c.rw.RLock()
	state := c.state
	c.rw.RUnlock()

	if state == cluster.Shut {
		c.events[event] = handler
	} else {
		log.Warnf("the client is working, can't add event handler")
	}
}
