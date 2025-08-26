package tcp

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
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
	var tempDelay time.Duration

	for {
		conn, err := s.listener.Accept()
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
			return
		}

		tempDelay = 0

		if err = s.connMgr.allocate(conn); err != nil {
			log.Errorf("connection allocate error: %v", err)
			_ = conn.Close()
		}
	}
}
