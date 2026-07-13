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
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
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

	g.refreshServiceInstance()

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
	g.proxy.trigger(g.ctx, cluster.Connect, conn.ID(), conn.UID())
}

// 处理断开连接
func (g *Gate) handleDisconnect(conn network.Conn) {
	g.session.RemConn(conn)

	cid, uid := conn.ID(), conn.UID()

	if uid != 0 {
		ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
		_ = g.proxy.unbindGate(ctx, cid, uid)
		cancel()
	}

	g.proxy.trigger(g.ctx, cluster.Disconnect, cid, uid)

	g.wg.Done()
}

// 处理接收到的消息
func (g *Gate) handleReceive(conn network.Conn, data []byte) {
	g.proxy.deliver(g.ctx, conn.ID(), conn.UID(), data)
}

// 启动传输服务器
func (g *Gate) startLinkerServer() {
	linker, err := gate.NewServer(&provider{gate: g}, &gate.ServerOptions{
		Addr:   g.opts.addr,
		Expose: g.opts.expose,
	})
	if err != nil {
		log.Fatalf("link server create failed: %v", err)
	}

	g.linker = linker

	go func() {
		if err = g.linker.Start(); err != nil {
			log.Errorf("link server start failed: %v", err)
		}
	}()
}

// 停止传输服务器
func (g *Gate) stopLinkerServer() {
	if g.linker == nil {
		return
	}

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
		Metadata: g.opts.metadata,
	}

	if err := g.doRegisterServiceInstance(); err != nil {
		log.Fatalf("register cluster instance failed: %v", err)
	}
}

// 刷新服务实例状态
func (g *Gate) refreshServiceInstance() {
	if err := g.doRefreshServiceInstance(g.getState()); err != nil {
		log.Errorf("refresh cluster instance failed: %v", err)
	}
}

// 解注册服务实例
func (g *Gate) deregisterServiceInstance() {
	ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
	err := g.opts.registry.Deregister(ctx, g.instance)
	cancel()
	if err != nil {
		log.Errorf("deregister cluster instance failed: %v", err)
	}
}

// 执行注册操作
func (g *Gate) doRegisterServiceInstance() error {
	ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
	err := g.opts.registry.Register(ctx, g.instance)
	cancel()

	return err
}

// 刷新服务实例状态
func (g *Gate) doRefreshServiceInstance(state ...cluster.State) error {
	if len(state) > 0 {
		g.instance.State = state[0].String()
	}

	return g.doRegisterServiceInstance()
}

// 获取状态
func (g *Gate) getState() cluster.State {
	return cluster.State(g.state.Load())
}

// 更新状态（仅能在Work或Busy状态间切换）
func (g *Gate) setState(state cluster.State) error {
	if state > cluster.Busy {
		return errors.ErrIllegalOperation
	}

	switch curr := g.getState(); curr {
	case cluster.Work, cluster.Busy:
		if curr == state {
			return nil
		}

		if g.state.CompareAndSwap(int32(curr), int32(state)) {
			return g.doRefreshServiceInstance(state)
		} else {
			return errors.ErrIllegalOperation
		}
	default:
		return errors.ErrIllegalOperation
	}
}

// 打印组件信息
func (g *Gate) printInfo() {
	infos := make([]string, 0, 6)
	infos = append(infos, fmt.Sprintf("ID: %s", g.opts.id))
	infos = append(infos, fmt.Sprintf("Name: %s", g.Name()))
	infos = append(infos, fmt.Sprintf("Link: %s", g.linker.ExposeAddr()))
	infos = append(infos, fmt.Sprintf("Server: [%s] %s", g.opts.server.Protocol(), net.FulfillAddr(g.opts.server.Addr())))
	infos = append(infos, fmt.Sprintf("Locator: %s", g.opts.locator.Name()))
	infos = append(infos, fmt.Sprintf("Registry: %s", g.opts.registry.Name()))

	info.PrintBoxInfo("Gate", infos...)
}
