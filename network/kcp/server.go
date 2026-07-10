package kcp

import (
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/xtaci/kcp-go/v5"
)

type server struct {
	opts              *serverOptions            // 配置
	started           atomic.Bool               // 是否已启动
	listener          *kcp.Listener             // 监听器
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

// Start 启动服务器
func (s *server) Start() error {
	if err := s.init(); err != nil {
		return err
	}

	go s.serve()

	if s.startHandler != nil {
		s.startHandler()
	}

	return nil
}

// Stop 关闭服务器
func (s *server) Stop() error {
	if !s.started.Swap(false) {
		return errors.ErrIllegalOperation
	}

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return err
		}
		s.listener = nil
	}

	s.connMgr.close()

	if s.stopHandler != nil {
		s.stopHandler()
	}

	return nil
}

// Protocol 协议
func (s *server) Protocol() string {
	return protocol
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

// 初始化服务器
func (s *server) init() error {
	if s.started.Swap(true) {
		return errors.ErrIllegalOperation
	}

	defer func() {
		if s.listener == nil {
			s.started.Store(false)
		}
	}()

	ln, err := kcp.ListenWithOptions(s.opts.addr, nil, 0, 0)
	if err != nil {
		return err
	}

	s.listener = ln

	return nil
}

// 启动服务器
func (s *server) serve() {
	listener := s.listener

	for {
		conn, err := listener.AcceptKCP()
		if err != nil {
			log.Warnf("kcp accept error: %v", err)
			break
		}

		if err = s.connMgr.allocateConn(conn); err != nil {
			log.Errorf("connection allocate error: %v", err)
			_ = conn.Close()
		}
	}

	s.listener = nil

	if s.started.CompareAndSwap(true, false) {
		s.connMgr.close()

		if s.stopHandler != nil {
			s.stopHandler()
		}
	}
}
