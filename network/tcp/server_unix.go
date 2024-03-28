//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux
// +build darwin netbsd freebsd openbsd dragonfly linux

package tcp

import (
	"context"
	"github.com/cloudwego/netpoll"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xcall"
	"net"
)

type server struct {
	opts              *serverOptions            // 配置
	listener          net.Listener              // 监听器
	connMgr           *serverConnMgr            // 连接管理器
	startHandler      network.StartHandler      // 服务器启动hook函数
	stopHandler       network.CloseHandler      // 服务器关闭hook函数
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
}

var _ network.Server = &server{}

func NewServer(opts ...ServerOption) network.Server {
	o := defaultServerOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &server{}
	s.opts = o
	s.connMgr = newServerConnMgr(s)

	return s
}

// Addr 监听地址
func (s *server) Addr() string {
	return s.opts.addr
}

// Protocol 协议
func (s *server) Protocol() string {
	return protocol
}

// Start 启动服务器
func (s *server) Start() error {
	if err := s.init(); err != nil {
		return err
	}

	xcall.Go(s.serve)

	if s.startHandler != nil {
		s.startHandler()
	}

	return nil
}

// Stop 关闭服务器
func (s *server) Stop() error {
	if err := s.listener.Close(); err != nil {
		return err
	}

	s.connMgr.close()

	if s.stopHandler != nil {
		s.stopHandler()
	}

	return nil
}

// 初始化服务器
func (s *server) init() error {
	addr, err := net.ResolveTCPAddr(s.Protocol(), s.opts.addr)
	if err != nil {
		return err
	}

	listener, err := netpoll.CreateListener(addr.Network(), addr.String())
	if err != nil {
		return err
	}

	s.listener = listener

	return nil
}

// 接受请求
func (s *server) onRequest(ctx context.Context, conn netpoll.Connection) error {
	c, ok := s.connMgr.load(conn)
	if !ok {
		return errors.New("invalid connection")
	}

	return c.read()
}

// 打开连接
func (s *server) onConnect(ctx context.Context, conn netpoll.Connection) context.Context {
	if err := s.connMgr.allocate(conn); err != nil {
		log.Errorf("connection allocate error: %v", err)
		_ = conn.Close()
		return nil
	}

	return ctx
}

// 启动服务器
func (s *server) serve() {
	eventLoop, err := netpoll.NewEventLoop(s.onRequest, netpoll.WithOnConnect(s.onConnect))
	if err != nil {
		log.Fatalf("tcp server start failed: %v", err)
	}

	if err = eventLoop.Serve(s.listener); err != nil {
		log.Fatalf("tcp server start failed: %v", err)
	}
}

// OnStart 监听服务器启动
func (s *server) OnStart(handler network.StartHandler) {
	s.startHandler = handler
}

// OnStop 监听服务器关闭
func (s *server) OnStop(handler network.CloseHandler) {
	s.stopHandler = handler
}

// OnConnect 监听连接打开
func (s *server) OnConnect(handler network.ConnectHandler) {
	s.connectHandler = handler
}

// OnDisconnect 监听连接关闭
func (s *server) OnDisconnect(handler network.DisconnectHandler) {
	s.disconnectHandler = handler
}

// OnReceive 监听接收到消息
func (s *server) OnReceive(handler network.ReceiveHandler) {
	s.receiveHandler = handler
}
