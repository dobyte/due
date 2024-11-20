/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/7 1:19 上午
 * @Desc: 网关服务器
 */

package gate

import (
	"context"
	"fmt"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"sync"
	"sync/atomic"
)

type Gate struct {
	component.Base
	opts     *options
	ctx      context.Context
	cancel   context.CancelFunc
	state    atomic.Int32
	proxy    *proxy
	instance *registry.ServiceInstance
	session  *session.Session
	linker   *gate.Server
	wg       *sync.WaitGroup
}

func NewGate(opts ...Option) *Gate {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	g := &Gate{}
	g.opts = o
	g.ctx, g.cancel = context.WithCancel(o.ctx)
	g.proxy = newProxy(g)
	g.session = session.NewSession()
	g.state.Store(int32(cluster.Shut))
	g.wg = &sync.WaitGroup{}

	return g
}

// Name 组件名称
func (g *Gate) Name() string {
	return g.opts.name
}

// Init 初始化
func (g *Gate) Init() {
	if g.opts.id == "" {
		log.Fatal("instance id can not be empty")
	}

	if g.opts.server == nil {
		log.Fatal("server component is not injected")
	}

	if g.opts.locator == nil {
		log.Fatal("locator component is not injected")
	}

	if g.opts.registry == nil {
		log.Fatal("registry component is not injected")
	}
}

// Start 启动组件
func (g *Gate) Start() {
	if !g.state.CompareAndSwap(int32(cluster.Shut), int32(cluster.Work)) {
		return
	}

	g.startNetworkServer()

	g.startLinkerServer()

	g.registerServiceInstance()

	g.proxy.watch()

	g.printInfo()
}

// Close 关闭节点
func (g *Gate) Close() {
	if !g.state.CompareAndSwap(int32(cluster.Work), int32(cluster.Hang)) {
		if !g.state.CompareAndSwap(int32(cluster.Busy), int32(cluster.Hang)) {
			return
		}
	}

	g.registerServiceInstance()

	g.wg.Wait()
}

// Destroy 销毁组件
func (g *Gate) Destroy() {
	if !g.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
		return
	}

	g.deregisterServiceInstance()

	g.stopNetworkServer()

	g.stopLinkerServer()

	g.cancel()
}

// 启动网络服务器
func (g *Gate) startNetworkServer() {
	g.opts.server.OnConnect(g.handleConnect)
	g.opts.server.OnDisconnect(g.handleDisconnect)
	g.opts.server.OnReceive(g.handleReceive)

	if err := g.opts.server.Start(); err != nil {
		log.Fatalf("network server start failed: %v", err)
	}
}

// 停止网关服务器
func (g *Gate) stopNetworkServer() {
	if err := g.opts.server.Stop(); err != nil {
		log.Errorf("network server stop failed: %v", err)
	}
}

// 处理连接打开
func (g *Gate) handleConnect(conn network.Conn) {
	g.wg.Add(1)

	g.session.AddConn(conn)

	cid, uid := conn.ID(), conn.UID()

	ctx, cancel := context.WithTimeout(g.ctx, g.opts.timeout)
	g.proxy.trigger(ctx, cluster.Connect, cid, uid)
	cancel()
}

// 处理断开连接
func (g *Gate) handleDisconnect(conn network.Conn) {
	g.session.RemConn(conn)

	if cid, uid := conn.ID(), conn.UID(); uid != 0 {
		ctx, cancel := context.WithTimeout(g.ctx, g.opts.timeout)
		_ = g.proxy.unbindGate(ctx, cid, uid)
		g.proxy.trigger(ctx, cluster.Disconnect, cid, uid)
		cancel()
	} else {
		ctx, cancel := context.WithTimeout(g.ctx, g.opts.timeout)
		g.proxy.trigger(ctx, cluster.Disconnect, cid, uid)
		cancel()
	}

	g.wg.Done()
}

// 处理接收到的消息
func (g *Gate) handleReceive(conn network.Conn, data []byte) {
	cid, uid := conn.ID(), conn.UID()
	ctx, cancel := context.WithTimeout(g.ctx, g.opts.timeout)
	g.proxy.deliver(ctx, cid, uid, data)
	cancel()
}

// 启动传输服务器
func (g *Gate) startLinkerServer() {
	transporter, err := gate.NewServer(g.opts.addr, &provider{gate: g})
	if err != nil {
		log.Fatalf("link server create failed: %v", err)
	}

	g.linker = transporter

	go func() {
		if err = g.linker.Start(); err != nil {
			log.Errorf("link server start failed: %v", err)
		}
	}()
}

// 停止传输服务器
func (g *Gate) stopLinkerServer() {
	if err := g.linker.Stop(); err != nil {
		log.Errorf("link server stop failed: %v", err)
	}
}

// 注册服务实例
func (g *Gate) registerServiceInstance() {
	g.instance = &registry.ServiceInstance{
		ID:       g.opts.id,
		Name:     cluster.Gate.String(),
		Kind:     cluster.Gate.String(),
		Alias:    g.opts.name,
		State:    g.getState().String(),
		Endpoint: g.linker.Endpoint().String(),
	}

	ctx, cancel := context.WithTimeout(g.ctx, defaultTimeout)
	defer cancel()

	if err := g.opts.registry.Register(ctx, g.instance); err != nil {
		log.Fatalf("register cluster instance failed: %v", err)
	}
}

// 解注册服务实例
func (g *Gate) deregisterServiceInstance() {
	ctx, cancel := context.WithTimeout(g.ctx, defaultTimeout)
	defer cancel()

	if err := g.opts.registry.Deregister(ctx, g.instance); err != nil {
		log.Errorf("deregister cluster instance failed: %v", err)
	}
}

// 获取状态
func (g *Gate) getState() cluster.State {
	return cluster.State(g.state.Load())
}

// 打印组件信息
func (g *Gate) printInfo() {
	infos := make([]string, 0)
	infos = append(infos, fmt.Sprintf("Name: %s", g.Name()))
	infos = append(infos, fmt.Sprintf("Link: %s", g.linker.ExposeAddr()))
	infos = append(infos, fmt.Sprintf("Server: [%s] %s", g.opts.server.Protocol(), net.FulfillAddr(g.opts.server.Addr())))
	infos = append(infos, fmt.Sprintf("Locator: %s", g.opts.locator.Name()))
	infos = append(infos, fmt.Sprintf("Registry: %s", g.opts.registry.Name()))

	info.PrintBoxInfo("Gate", infos...)
}
