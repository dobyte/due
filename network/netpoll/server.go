package netpoll

import (
	"context"
	"github.com/cloudwego/netpoll"
	"github.com/libp2p/go-reuseport"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/network"
	"net"
	"time"
)

type server struct {
	opts              *serverOptions            // 配置
	listeners         [10]net.Listener          // 监听器
	connMgr           *connMgr                  // 连接管理器
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
	s.connMgr = newConnMgr(s)

	return s
}

// Addr 监听地址
func (s *server) Addr() string {
	return s.opts.addr
}

// Protocol 协议
func (s *server) Protocol() string {
	return "tcp"
}

// Start 启动服务器
func (s *server) Start() error {
	if err := s.init(); err != nil {
		return err
	}

	if s.startHandler != nil {
		s.startHandler()
	}

	go s.serve()

	return nil
}

// Stop 关闭服务器
func (s *server) Stop() (err error) {
	for _, ln := range s.listeners {
		if ln != nil {
			if e := ln.Close(); e != nil {
				err = e
			}
		}
	}

	s.connMgr.close()

	return
}

func (s *server) init() error {
	addr, err := net.ResolveTCPAddr(s.Protocol(), s.opts.addr)
	if err != nil {
		return err
	}

	for i := 0; i < len(s.listeners); i++ {
		ln, err := reuseport.Listen(addr.Network(), addr.String())
		if err != nil {
			for n := 0; n < i; n++ {
				s.listeners[n].Close()
			}
			return err
		}

		s.listeners[i] = ln
	}

	return nil
}

func (s *server) onRequest(ctx context.Context, conn netpoll.Connection) error {
	c, ok := s.connMgr.load(conn)
	if !ok {
		return errors.New("invalid connection")
	}

	return c.read()
}

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
	for i := range s.listeners {
		eventLoop, err := netpoll.NewEventLoop(
			s.onRequest,
			netpoll.WithOnConnect(s.onConnect),
			netpoll.WithReadTimeout(time.Second),
		)
		if err != nil {
			log.Fatalf("tcp server start failed: %v", err)
		}

		if err = eventLoop.Serve(s.listeners[i]); err != nil {
			log.Fatalf("tcp server start failed: %v", err)
		}
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
