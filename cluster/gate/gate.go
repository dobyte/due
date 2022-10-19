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
	"sync"
	"time"

	"github.com/dobyte/due/internal/xnet"
	"github.com/dobyte/due/packet"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/router"
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
	router   *router.Router
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
	g.router = router.NewRouter()
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
		log.Fatal("server plugin is not injected")
	}
	if g.opts.locator == nil {
		log.Fatal("locator plugin is not injected")
	}
	if g.opts.registry == nil {
		log.Fatal("registry plugin is not injected")
	}
	if g.opts.transporter == nil {
		log.Fatal("transporter plugin is not injected")
	}
}

// Start 启动组件
func (g *Gate) Start() {
	g.startRPCServer()

	g.startGateServer()

	g.registerInstance()

	g.proxy.watch(g.ctx)

	g.debugPrint()
}

// Destroy 销毁组件
func (g *Gate) Destroy() {
	g.deregisterInstance()

	g.stopRPCServer()

	g.stopGateServer()

	g.cancel()
}

// 启动网关服务器
func (g *Gate) startGateServer() {
	g.opts.server.OnConnect(g.handleConnect)
	g.opts.server.OnDisconnect(g.handleDisconnect)
	g.opts.server.OnReceive(g.handleReceive)

	if err := g.opts.server.Start(); err != nil {
		log.Fatalf("the gate server startup failed: %v", err)
	}
}

// 停止网关服务器
func (g *Gate) stopGateServer() {
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
func (g *Gate) startRPCServer() {
	var err error

	g.rpc, err = g.opts.transporter.NewGateServer(&provider{g})
	if err != nil {
		log.Fatalf("the rpc server build failed: %v", err)
	}

	go func() {
		if err = g.rpc.Start(); err != nil {
			log.Fatalf("the rpc server startup failed: %v", err)
		}
	}()
}

// 停止RPC服务器
func (g *Gate) stopRPCServer() {
	if err := g.rpc.Stop(); err != nil {
		log.Errorf("the rpc server stop failed: %v", err)
	}
}

// 注册服务实例
func (g *Gate) registerInstance() {
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

	ctx, cancel = context.WithTimeout(g.ctx, 10*time.Second)
	watcher, err := g.opts.registry.Watch(ctx, string(cluster.Node))
	cancel()
	if err != nil {
		log.Fatalf("the node service instances watch failed: %v", err)
	}

	go func() {
		defer watcher.Stop()

		for {
			select {
			case <-g.ctx.Done():
				return
			default:
				// exec watch
			}

			services, err := watcher.Next()
			if err != nil {
				continue
			}
			g.router.ReplaceServices(services...)
		}
	}()
}

// 解注册服务实例
func (g *Gate) deregisterInstance() {
	ctx, cancel := context.WithTimeout(g.ctx, 10*time.Second)
	err := g.opts.registry.Deregister(ctx, g.instance)
	defer cancel()
	if err != nil {
		log.Errorf("the gate service instance deregister failed: %v", err)
	}
}

func (g *Gate) debugPrint() {
	log.Debugf("The gate server startup successful")
	log.Debugf("Gate server, listen: %s protocol: %s", xnet.FulfillAddr(g.opts.server.Addr()), g.opts.server.Protocol())
	log.Debugf("RPC  server, listen: %s protocol: %s", xnet.FulfillAddr(g.rpc.Addr()), g.rpc.Scheme())
}
