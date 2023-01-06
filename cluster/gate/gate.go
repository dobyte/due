/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/7 1:19 上午
 * @Desc: 网关服务器
 */

package gate

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/utils/xnet"
	"sync"
	"time"

	"github.com/dobyte/due/packet"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/session"

	"github.com/dobyte/due/component"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/network"
)

type Gate struct {
	component.Base
	opts     *options
	ctx      context.Context
	cancel   context.CancelFunc
	group    *session.Group
	sessions sync.Pool
	proxy    *proxy
	instance *registry.ServiceInstance
	rpc      transport.Server
}

func NewGate(opts ...Option) *Gate {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	g := &Gate{}
	g.opts = o
	g.group = session.NewGroup()
	g.proxy = newProxy(g)
	g.sessions.New = func() interface{} { return session.NewSession() }
	g.ctx, g.cancel = context.WithCancel(o.ctx)

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

	if g.opts.transporter == nil {
		log.Fatal("transporter component is not injected")
	}
}

// Start 启动组件
func (g *Gate) Start() {
	g.startNetworkServer()

	g.startTransportServer()

	g.registerServiceInstance()

	g.proxy.watch(g.ctx)

	g.debugPrint()
}

// Destroy 销毁组件
func (g *Gate) Destroy() {
	g.deregisterServiceInstance()

	g.stopNetworkServer()

	g.stopTransportServer()

	g.cancel()
}

// 启动网络服务器
func (g *Gate) startNetworkServer() {
	g.opts.server.OnConnect(g.handleConnect)
	g.opts.server.OnDisconnect(g.handleDisconnect)
	g.opts.server.OnReceive(g.handleReceive)

	go func() {
		if err := g.opts.server.Start(); err != nil {
			log.Fatalf("the gate server startup failed: %v", err)
		}
	}()
}

// 停止网关服务器
func (g *Gate) stopNetworkServer() {
	if err := g.opts.server.Stop(); err != nil {
		log.Errorf("the gate server stop failed: %v", err)
	}
}

// 处理连接打开
func (g *Gate) handleConnect(conn network.Conn) {
	s := g.sessions.Get().(*session.Session)
	s.Init(conn)
	g.group.AddSession(s)
}

// 处理断开连接
func (g *Gate) handleDisconnect(conn network.Conn) {
	s, err := g.group.RemSession(session.Conn, conn.ID())
	if err != nil {
		log.Errorf("session remove failed, gid: %d, cid: %d, uid: %d, err: %v", g.opts.id, s.CID(), s.UID(), err)
		return
	}

	if uid := conn.UID(); uid > 0 {
		ctx, cancel := context.WithTimeout(g.ctx, g.opts.timeout)
		err = g.proxy.unbindGate(ctx, uid)
		cancel()
		if err != nil {
			log.Errorf("user unbind failed, gid: %d, uid: %d, err: %v", g.opts.id, uid, err)
		}
	}

	s.Reset()
	g.sessions.Put(s)
}

// 处理接收到的消息
func (g *Gate) handleReceive(conn network.Conn, data []byte, _ int) {
	message, err := packet.Unpack(data)
	if err != nil {
		log.Errorf("unpack data to struct failed: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(g.ctx, g.opts.timeout)
	err = g.proxy.deliver(ctx, conn.ID(), conn.UID(), message)
	cancel()
	if err != nil {
		log.Errorf("deliver message failed: %v", err)
	}
}

// 启动RPC服务器
func (g *Gate) startTransportServer() {
	var err error

	g.rpc, err = g.opts.transporter.NewGateServer(&provider{g})
	if err != nil {
		log.Fatalf("the transport server build failed: %v", err)
	}

	go func() {
		if err = g.rpc.Start(); err != nil {
			log.Fatalf("the transport server startup failed: %v", err)
		}
	}()
}

// 停止RPC服务器
func (g *Gate) stopTransportServer() {
	if err := g.rpc.Stop(); err != nil {
		log.Errorf("the transport server stop failed: %v", err)
	}
}

// 注册服务实例
func (g *Gate) registerServiceInstance() {
	g.instance = &registry.ServiceInstance{
		ID:       g.opts.id,
		Name:     string(cluster.Gate),
		Kind:     cluster.Gate,
		Alias:    g.opts.name,
		State:    cluster.Work,
		Endpoint: g.rpc.Endpoint().String(),
	}

	ctx, cancel := context.WithTimeout(g.ctx, 10*time.Second)
	err := g.opts.registry.Register(ctx, g.instance)
	cancel()
	if err != nil {
		log.Fatalf("the gate service instance register failed: %v", err)
	}
}

// 解注册服务实例
func (g *Gate) deregisterServiceInstance() {
	ctx, cancel := context.WithTimeout(g.ctx, 10*time.Second)
	err := g.opts.registry.Deregister(ctx, g.instance)
	defer cancel()
	if err != nil {
		log.Errorf("the gate service instance deregister failed: %v", err)
	}
}

func (g *Gate) debugPrint() {
	log.Debugf("The gate server startup successful")
	log.Debugf("Network server, listen: %s protocol: %s", xnet.FulfillAddr(g.opts.server.Addr()), g.opts.server.Protocol())
	log.Debugf("Transport server, listen: %s protocol: %s", xnet.FulfillAddr(g.rpc.Addr()), g.rpc.Scheme())
}
