package client

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xcall"
)

// HookHandler 客户端钩子处理函数
type HookHandler func(proxy *Proxy)

// RouteHandler 客户端路由处理函数
type RouteHandler func(ctx *Context)

// EventHandler 客户端事件处理函数
type EventHandler func(conn *Conn)

// Client 客户端组件
// 负责与服务端建立连接，并提供路由、事件、钩子的注册与消息收发能力
type Client struct {
	component.Base
	opts                *options           // 配置项
	ctx                 context.Context    // 上下文
	cancel              context.CancelFunc // 取消函数
	proxy               *Proxy             // 客户端代理
	state               atomic.Int32       // 客户端状态
	rw1                 sync.Mutex         // 注册锁，保护路由/事件/钩子的并发注册
	hooks               atomic.Value       // 钩子处理器集合（map[cluster.Hook][]HookHandler）
	routes              atomic.Value       // 路由处理器集合（map[int32][]RouteHandler）
	events              atomic.Value       // 事件处理器集合（map[cluster.Event][]EventHandler）
	defaultRouteHandler atomic.Value       // 默认路由处理器（RouteHandler）
	rw2                 sync.RWMutex       // 连接锁，保护连接表的并发读写
	conns               sync.Map           // 连接表（network.Conn -> *Conn）
}

// NewClient 创建客户端组件
// @param opts ...Option 客户端配置项
// @return @1 *Client 客户端组件实例
func NewClient(opts ...Option) *Client {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &Client{}
	c.opts = o
	c.proxy = newProxy(c)
	c.hooks.Store(make(map[cluster.Hook][]HookHandler))
	c.routes.Store(make(map[int32][]RouteHandler))
	c.events.Store(make(map[cluster.Event][]EventHandler))
	c.defaultRouteHandler.Store(RouteHandler(nil))
	c.ctx, c.cancel = context.WithCancel(o.ctx)
	c.state.Store(int32(cluster.Shut))

	return c
}

// Name 获取客户端名称
// @return @1 string 客户端名称
func (c *Client) Name() string {
	return c.opts.name
}

// Init 初始化客户端
// 校验网络客户端与编解码器等必要配置，缺失时直接终止进程，并触发Init钩子
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
	c.rw1.Lock()

	if !c.state.CompareAndSwap(int32(cluster.Shut), int32(cluster.Work)) {
		c.rw1.Unlock()
		return
	}

	c.opts.client.OnDisconnect(c.handleDisconnect)
	c.opts.client.OnReceive(c.handleReceive)

	c.rw1.Unlock()

	c.printInfo()

	c.runHookFunc(cluster.Start)
}

// Close 关闭节点
func (c *Client) Close() {
	if !c.state.CompareAndSwap(int32(cluster.Work), int32(cluster.Hang)) {
		if !c.state.CompareAndSwap(int32(cluster.Busy), int32(cluster.Hang)) {
			return
		}
	}

	c.runHookFunc(cluster.Close)
}

// Destroy 销毁客户端
// 将状态置为关闭，关闭全部连接并触发Destroy钩子
func (c *Client) Destroy() {
	if !c.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
		return
	}

	c.cancel()

	conns := make([]*Conn, 0)

	c.rw2.Lock()
	c.conns.Range(func(conn, _ any) bool {
		conns = append(conns, conn.(*Conn))
		return true
	})
	c.conns.Clear()
	c.rw2.Unlock()

	for _, cc := range conns {
		cc.Close()
	}

	c.runHookFunc(cluster.Destroy)
}

// Proxy 获取客户端代理
// @return @1 *Proxy 客户端代理
func (c *Client) Proxy() *Proxy {
	return c.proxy
}

// 处理断开连接
// 从连接表中移除连接并触发断开事件
// @param conn network.Conn 已断开的网络连接
func (c *Client) handleDisconnect(conn network.Conn) {
	val, ok := c.conns.Load(conn)
	if !ok {
		return
	}

	c.conns.Delete(conn)

	if handlers, ok := c.events.Load().(map[cluster.Event][]EventHandler)[cluster.Disconnect]; ok {
		for _, handler := range handlers {
			xcall.Call(func() {
				handler(val.(*Conn))
			})
		}
	}
}

// 处理接收到的消息
// 解包消息后分发给对应的路由处理器，未注册路由时走默认路由处理器
// @param conn network.Conn 消息来源连接
// @param data []byte 原始消息内容
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

	if handlers, ok := c.routes.Load().(map[int32][]RouteHandler)[message.Route]; ok {
		for _, handler := range handlers {
			xcall.Call(func() {
				handler(&Context{
					ctx:     context.Background(),
					conn:    val.(*Conn),
					message: message,
				})
			})
		}
	} else if handler := c.defaultRouteHandler.Load().(RouteHandler); handler != nil {
		xcall.Call(func() {
			handler(&Context{
				ctx:     context.Background(),
				conn:    val.(*Conn),
				message: message,
			})
		})
	} else {
		log.Debugf("route handler is not registered, route: %v", message.Route)
	}
}

// 拨号
func (c *Client) dial(opts ...DialOption) (*Conn, error) {
	if st := c.getState(); st != cluster.Work && st != cluster.Busy {
		return nil, errors.ErrClientShut
	}

	o := &dialOptions{attrs: make(map[string]any)}
	for _, opt := range opts {
		opt(o)
	}

	conn, err := c.opts.client.Dial(o.addr)
	if err != nil {
		return nil, err
	}

	cc := &Conn{conn: conn, client: c}

	for key, value := range o.attrs {
		cc.SetAttr(key, value)
	}

	c.rw2.Lock()
	if st := c.getState(); st != cluster.Work && st != cluster.Busy {
		c.rw2.Unlock()
		conn.Close()
		return nil, errors.ErrClientShut
	}
	c.conns.Store(conn, cc)
	c.rw2.Unlock()

	if handlers, ok := c.events.Load().(map[cluster.Event][]EventHandler)[cluster.Connect]; ok {
		for _, handler := range handlers {
			xcall.Call(func() {
				handler(cc)
			})
		}
	}

	return cc, nil
}

// 添加事件处理器
// @param event cluster.Event 事件类型
// @param handler EventHandler 事件处理函数
func (c *Client) addEventListener(event cluster.Event, handler EventHandler) {
	c.rw1.Lock()
	defer c.rw1.Unlock()

	if c.getState() != cluster.Shut {
		log.Warnf("client is working, can't add event handler")
		return
	}

	oldEvents := c.events.Load().(map[cluster.Event][]EventHandler)
	newEvents := maps.Clone(oldEvents)
	newEvents[event] = append(newEvents[event], handler)

	c.events.Store(newEvents)
}

// 添加路由处理器
// @param route int32 路由号
// @param handler RouteHandler 路由处理函数
func (c *Client) addRouteHandler(route int32, handler RouteHandler) {
	c.rw1.Lock()
	defer c.rw1.Unlock()

	if c.getState() != cluster.Shut {
		log.Warnf("client is working, can't add route handler")
		return
	}

	oldRoutes := c.routes.Load().(map[int32][]RouteHandler)
	newRoutes := maps.Clone(oldRoutes)
	newRoutes[route] = append(newRoutes[route], handler)

	c.routes.Store(newRoutes)
}

// 默认路由处理器
// 设置默认路由处理器，所有未注册的路由均走默认路由处理器；仅允许设置一次
// @param handler RouteHandler 默认路由处理函数
func (c *Client) setDefaultRouteHandler(handler RouteHandler) {
	c.rw1.Lock()
	defer c.rw1.Unlock()

	if c.getState() != cluster.Shut {
		log.Warnf("client is working, can't set default route handler")
		return
	}

	if cur := c.defaultRouteHandler.Load().(RouteHandler); cur != nil {
		log.Warnf("default route handler is already set")
		return
	}

	c.defaultRouteHandler.Store(handler)
}

// 添加钩子监听器
func (c *Client) addHookListener(hook cluster.Hook, handler HookHandler) {
	c.rw1.Lock()
	defer c.rw1.Unlock()

	if hook != cluster.Destroy && c.getState() != cluster.Shut {
		log.Warnf("client is working, can't add hook handler")
		return
	}

	oldHooks := c.hooks.Load().(map[cluster.Hook][]HookHandler)
	newHooks := maps.Clone(oldHooks)
	newHooks[hook] = append(newHooks[hook], handler)

	c.hooks.Store(newHooks)
}

// 获取客户端状态
// @return @1 cluster.State 客户端状态
func (c *Client) getState() cluster.State {
	return cluster.State(c.state.Load())
}

// 执行钩子函数
// 触发指定钩子对应的全部监听器，并等待所有监听器执行完成
// @param hook cluster.Hook 钩子类型
func (c *Client) runHookFunc(hook cluster.Hook) {
	handlers, ok := c.hooks.Load().(map[cluster.Hook][]HookHandler)[hook]
	if !ok {
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(handlers))

	for i := range handlers {
		handler := handlers[i]
		xcall.Go(func() {
			handler(c.proxy)
			wg.Done()
		})
	}

	wg.Wait()
}

// 打印组件信息
// 输出客户端名称、编解码器、协议与加密器等基础信息
func (c *Client) printInfo() {
	infos := make([]string, 0)
	infos = append(infos, fmt.Sprintf("Name: %s", c.Name()))
	infos = append(infos, fmt.Sprintf("Codec: %s", c.opts.codec.Name()))
	infos = append(infos, fmt.Sprintf("Protocol: %s", c.opts.client.Protocol()))

	if c.opts.encryptor != nil {
		infos = append(infos, fmt.Sprintf("Encryptor: %s", c.opts.encryptor.Name()))
	} else {
		infos = append(infos, "Encryptor: -")
	}

	info.PrintBoxInfo("Client", infos...)
}
