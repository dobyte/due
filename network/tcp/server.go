package tcp

import (
	"crypto/tls"
	"net"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xcall"
)

type server struct {
	opts              *serverOptions            // 配置
	started           atomic.Bool               // 是否已启动
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
	if !s.started.Swap(false) {
		return errors.ErrIllegalOperation
	}

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return err
		}
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

// 初始化TCP服务器
func (s *server) init() error {
	if s.started.Swap(true) {
		return errors.ErrIllegalOperation
	}

	defer func() {
		if s.listener == nil {
			s.started.Store(false)
		}
	}()

	addr, err := net.ResolveTCPAddr("tcp", s.opts.addr)
	if err != nil {
		return err
	}

	if s.opts.certFile != "" && s.opts.keyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.opts.certFile, s.opts.keyFile)
		if err != nil {
			return err
		}

		if s.listener, err = tls.Listen(addr.Network(), addr.String(), &tls.Config{
			Certificates: []tls.Certificate{cert},
		}); err != nil {
			return err
		}
	} else {
		if s.listener, err = net.ListenTCP(addr.Network(), addr); err != nil {
			return err
		}
	}

	return nil
}

// 等待连接
func (s *server) serve() {
	var (
		listener  = s.listener
		tempDelay time.Duration
	)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}

				log.Warnf("tcp accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}

			log.Warnf("tcp accept error: %v", err)
			break
		}

		tempDelay = 0

		conn.(*net.TCPConn).SetNoDelay(true)

		if err = s.connMgr.allocateConn(conn); err != nil {
			log.Errorf("connection allocate error: %v", err)
			_ = conn.Close()
		}
	}

	if s.started.CompareAndSwap(true, false) {
		s.connMgr.close()

		if s.stopHandler != nil {
			s.stopHandler()
		}
	}
}
