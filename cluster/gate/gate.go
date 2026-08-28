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

// NewGate 创建一个网关服务器组件实例
// @param opts ...Option 配置项
// @return @1 *Gate 网关服务器组件实例
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

// Name 获取组件名称
// @return @1 string 组件名称
func (g *Gate) Name() string {
	return g.opts.name
}

// Init 初始化组件
// 校验实例ID及必要组件（服务器、定位器、服务注册器）是否注入。
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
// 依次启动网络服务器与内网链接服务器、注册服务实例、监听用户定位与集群实例，并打印组件信息
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
// 切换状态为Hang并刷新服务实例状态，随后等待所有在线的客户端会话自然退出（优雅等待语义）
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
// 依次解注册服务实例、停止网络服务器与链接服务器，并取消组件上下文
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
// 注册连接打开、断开与接收处理器并启动网络服务器，启动失败时记录致命错误
func (g *Gate) startNetworkServer() {
	g.opts.server.OnConnect(g.handleConnect)
	g.opts.server.OnDisconnect(g.handleDisconnect)
	g.opts.server.OnReceive(g.handleReceive)

	if err := g.opts.server.Start(); err != nil {
		log.Fatalf("network server start failed: %v", err)
	}
}

// 停止网关服务器
// 关闭网络服务器，失败时记录错误日志
func (g *Gate) stopNetworkServer() {
	if err := g.opts.server.Stop(); err != nil {
		log.Errorf("network server stop failed: %v", err)
	}
}

// 处理连接打开
// 仅在服务器处于Work或Busy状态时接受连接并登记会话；其他状态（Hang/Shut）下直接关闭连接拒绝接入
// @param conn network.Conn 新建立的连接
func (g *Gate) handleConnect(conn network.Conn) {
	if state := cluster.State(g.state.Load()); state == cluster.Work || state == cluster.Busy {
		g.wg.Add(1)
		g.session.AddConn(conn)
		g.proxy.trigger(g.ctx, cluster.Connect, conn.ID(), conn.UID())
	} else {
		if err := conn.Close(); err != nil {
			log.Warnf("close conn failed: %v", err)
		}
	}
}

// 处理断开连接
// 对已登记会话的连接执行解绑用户、触发Disconnect事件并完成WaitGroup计数；未登记（关闭阶段被拒绝接入）的连接直接忽略
// @param conn network.Conn 断开的连接
func (g *Gate) handleDisconnect(conn network.Conn) {
	cid := conn.ID()

	if ok, _ := g.session.Has(session.Conn, cid); ok {
		g.session.RemConn(conn)

		uid := conn.UID()

		if uid != 0 {
			ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
			_ = g.proxy.unbindGate(ctx, cid, uid)
			cancel()
		}

		g.proxy.trigger(g.ctx, cluster.Disconnect, cid, uid)

		g.wg.Done()
	}
}

// 处理接收到的消息
// 将客户端消息投递到对应业务节点
// @param conn network.Conn 来源连接
// @param data []byte 原始消息内容
func (g *Gate) handleReceive(conn network.Conn, data []byte) {
	g.proxy.deliver(g.ctx, conn.ID(), conn.UID(), data)
}

// 启动传输服务器
// 创建并异步启动内网RPC链接服务器
func (g *Gate) startLinkerServer() {
	linker, err := gate.NewServer(&provider{gate: g}, &gate.ServerOptions{
		Addr:   g.opts.addr,
		Expose: g.opts.expose,
	})
	if err != nil {
		log.Fatalf("linker server create failed: %v", err)
	}

	g.linker = linker

	go func() {
		if err = g.linker.Start(); err != nil {
			log.Fatalf("linker server start failed: %v", err)
		}
	}()
}

// 停止传输服务器
// 关闭内网RPC链接服务器，失败时记录错误日志
func (g *Gate) stopLinkerServer() {
	if g.linker == nil {
		return
	}

	if err := g.linker.Stop(); err != nil {
		log.Errorf("linker server stop failed: %v", err)
	}
}

// 注册服务实例
// 构造网关服务实例信息并注册到服务注册中心
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
// 使用当前状态重新注册服务实例
func (g *Gate) refreshServiceInstance() {
	if err := g.doRefreshServiceInstance(g.getState()); err != nil {
		log.Errorf("refresh cluster instance failed: %v", err)
	}
}

// 解注册服务实例
// 从服务注册中心移除本网关服务实例
func (g *Gate) deregisterServiceInstance() {
	ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
	err := g.opts.registry.Deregister(ctx, g.instance)
	cancel()
	if err != nil {
		log.Errorf("deregister cluster instance failed: %v", err)
	}
}

// 执行注册操作
// 带超时地向服务注册中心注册服务实例
// @return @1 error 错误信息
func (g *Gate) doRegisterServiceInstance() error {
	ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
	err := g.opts.registry.Register(ctx, g.instance)
	cancel()

	return err
}

// 刷新服务实例状态
// 更新实例状态字段后重新注册服务实例
// @param state ...cluster.State 可选，需更新的目标状态
// @return @1 error 错误信息
func (g *Gate) doRefreshServiceInstance(state ...cluster.State) error {
	if len(state) > 0 {
		g.instance.State = state[0].String()
	}

	return g.doRegisterServiceInstance()
}

// 获取状态
// @return @1 cluster.State 当前状态
func (g *Gate) getState() cluster.State {
	return cluster.State(g.state.Load())
}

// 更新状态（仅能在Work或Busy状态间切换）
// 状态切换成功后以新状态刷新服务实例
// @param state cluster.State 目标状态
// @return @1 error 错误信息
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

// 是否已关闭
// @return @1 bool 当前状态是否为关闭
func (g *Gate) isShut() bool {
	return g.getState() == cluster.Shut
}

// 打印组件信息
// 以信息框形式输出网关节点的各项配置
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
