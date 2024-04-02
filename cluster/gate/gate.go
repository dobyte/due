/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/7 1:19 上午
 * @Desc: 网关服务器
 */

package gate

import (
	"context"
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/common/link"
	"github.com/symsimmy/due/metrics/prometheus"
	"github.com/symsimmy/due/session"
	"github.com/symsimmy/due/transport"
	"strconv"
	"time"

	"github.com/symsimmy/due/component"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/network"
	"github.com/symsimmy/due/registry"
)

type Gate struct {
	component.Base
	opts     *options
	ctx      context.Context
	cancel   context.CancelFunc
	proxy    *Proxy
	instance *registry.ServiceInstance
	rpc      transport.Server
	session  *session.Session
}

func NewGate(opts ...Option) *Gate {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	g := &Gate{}
	g.opts = o
	g.proxy = newProxy(g)
	g.session = session.NewSession()
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

	g.startRPCServer()

	g.startPromServer()

	g.startCatServer()

	g.registerServiceInstance()

	g.proxy.watch(g.ctx)

	g.debugPrint()
}

// Destroy 销毁组件
func (g *Gate) Destroy() {
	g.deregisterServiceInstance()

	g.stopNetworkServer()

	g.stopRPCServer()

	g.stopPromServer()

	g.stopCatServer()

	g.cancel()
}

// Proxy 获取节点代理
func (n *Gate) Proxy() *Proxy {
	return n.proxy
}

func (n *Gate) startPromServer() {
	n.opts.promServer.Start()
}

func (n *Gate) stopPromServer() {
	n.opts.promServer.Destroy()
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
	// track gate online num
	prometheus.GateServerTotalOnlinePlayerGauge.WithLabelValues(g.opts.id).Inc()

	g.session.AddConn(conn)

	cid, uid := conn.ID(), conn.UID()
	ctx, cancel := context.WithTimeout(g.ctx, g.opts.timeout)
	g.proxy.trigger(ctx, cluster.Connect, cid, uid)
	cancel()
}

// 处理断开连接
func (g *Gate) handleDisconnect(conn network.Conn) {
	// track gate online num

	cid, uid := conn.ID(), conn.UID()
	ctx, cancel := context.WithTimeout(g.ctx, g.opts.timeout)
	if userGateId, err := g.proxy.link.LocateGate(ctx, uid); err == nil {
		// 判断uid 是否在当前 gate，如果不是则不发 disconnect 消息给 game server
		if userGateId != g.opts.id {
			log.Warnf("cid:%+v,uid:%+v login on another gate:%+v not this one:%+v,remove conn", cid, uid, userGateId, g.opts.id)
			g.session.RemConn(conn)
			cancel()
			return
		} else {
			// 如果是在当前 gate 上，判断是否是已销毁的连接发送的断开连接消息
			// 如果 cid < session中 conn 的 cid，说明是过期连接
			// 不处理跳过
			existsConn, err := g.session.Conn(session.User, conn.UID())
			if err != nil {
				log.Warnf("cid:%+v,uid:%+v get conn from session failed.err:%+v", conn.ID(), conn.UID(), err)
				g.session.RemConn(conn)
				cancel()
				return
			}

			if existsConn.ID() > conn.ID() {
				log.Warnf("uid:%+v disconnecting conn cid:%+v < exists conn cid:%+v,just skip.", conn.UID(), cid, existsConn.ID())
				g.session.RemConn(conn)
				cancel()
				return
			}
		}

	} else if errors.Is(err, link.ErrGateNotFoundUserSource) {

	} else {
		log.Warnf("cid:%+v,uid:%+v handleDisconnect locate gate failed.err:%+v", cid, uid, err)
	}

	g.session.RemConn(conn)

	if uid != 0 {
		log.Infof("cid:%+v,uid:%+v unbind gate %+v", cid, uid, g.opts.id)
		_ = g.proxy.unbindGate(ctx, cid, uid)
		g.proxy.trigger(ctx, cluster.Disconnect, cid, uid)
		cancel()
	} else {
		g.proxy.trigger(ctx, cluster.Disconnect, cid, uid)
		cancel()
	}
	if conn.UID() > 0 {
		log.Debugf("connection disconnected.cid:%+v, uid:%+v", conn.ID(), conn.UID())
	}
}

// 处理接收到的消息
func (g *Gate) handleReceive(conn network.Conn, data []byte, _ int) {
	cid, uid := conn.ID(), conn.UID()
	ctx, cancel := context.WithTimeout(g.ctx, g.opts.timeout)
	g.proxy.deliver(ctx, cid, uid, data)
	cancel()
}

// 启动RPC服务器
func (g *Gate) startRPCServer() {
	var err error

	g.rpc, err = g.opts.transporter.NewGateServer(&provider{g})
	if err != nil {
		log.Fatalf("rpc server create failed: %v", err)
	}

	go func() {
		if err = g.rpc.Start(); err != nil {
			log.Fatalf("rpc server start failed: %v", err)
		}
	}()
}

// 停止RPC服务器
func (g *Gate) stopRPCServer() {
	if err := g.rpc.Stop(); err != nil {
		log.Errorf("rpc server stop failed: %v", err)
	}
}

func (g *Gate) startCatServer() {
	if g.opts.catServer != nil {
		g.opts.catServer.Start()
	}
}

func (g *Gate) stopCatServer() {
	if g.opts.catServer != nil {
		g.opts.catServer.Destroy()
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
	if g.opts.promServer.Enable() {
		metricsPort, err := strconv.Atoi(g.opts.promServer.GetMetricsPort())
		if err != nil {
			panic(err)
		}
		g.instance.MetricsPort = metricsPort
	}

	ctx, cancel := context.WithTimeout(g.ctx, 10*time.Second)
	err := g.opts.registry.Register(ctx, g.instance)
	cancel()
	if err != nil {
		log.Fatalf("register dispatcher instance failed: %v", err)
	}
}

// 解注册服务实例
func (g *Gate) deregisterServiceInstance() {
	ctx, cancel := context.WithTimeout(g.ctx, 10*time.Second)
	err := g.opts.registry.Deregister(ctx, g.instance)
	defer cancel()
	if err != nil {
		log.Errorf("deregister dispatcher instance failed: %v", err)
	}
}

func (g *Gate) debugPrint() {
	log.Debugf("gate server startup successful")
	log.Debugf("%s server listen on %s", g.opts.server.Protocol(), g.opts.server.Addr())
	log.Debugf("%s server listen on %s", g.rpc.Scheme(), g.rpc.Addr())
}
