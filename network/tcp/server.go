package tcp

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/pires/go-proxyproto"
)

type server struct {
	opts              *serverOptions            // 配置
	mu                sync.Mutex                // 锁
	listener          net.Listener              // 监听器
	connMgr           *serverConnMgr            // 连接管理器
	startHandler      network.StartHandler      // 服务器启动hook函数
	stopHandler       network.CloseHandler      // 服务器关闭hook函数
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	heartbeatHandler  network.HeartbeatHandler  // 连接心跳hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
}

var _ network.Server = &server{}

// NewServer 创建一个服务器
// @param opts ...ServerOption 服务器配置项
// @return @1 network.Server 服务器实例
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

// Addr 获取监听地址
// @return @1 string 监听地址
func (s *server) Addr() string {
	return s.opts.addr
}

// Start 启动服务器
// @return @1 error 错误信息
func (s *server) Start() error {
	s.mu.Lock()

	if err := s.init(); err != nil {
		s.mu.Unlock()
		return err
	}

	ln := s.listener

	go s.serve(ln)

	s.mu.Unlock()

	if s.startHandler != nil {
		s.startHandler()
	}

	return nil
}

// Stop 关闭服务器
// @return @1 error 错误信息
func (s *server) Stop() error {
	s.mu.Lock()
	if s.listener != nil {
		s.listener.Close()
		s.listener = nil
	} else {
		s.mu.Unlock()
		return errors.ErrServerClosed
	}
	s.mu.Unlock()

	s.connMgr.close()

	if s.stopHandler != nil {
		s.stopHandler()
	}

	return nil
}

// Protocol 获取协议名称
// @return @1 string 协议名称
func (s *server) Protocol() string {
	return protocol
}

// OnStart 监听服务器启动
// @param handler network.StartHandler 服务器启动处理函数
func (s *server) OnStart(handler network.StartHandler) {
	s.startHandler = handler
}

// OnStop 监听服务器关闭
// @param handler network.CloseHandler 服务器关闭处理函数
func (s *server) OnStop(handler network.CloseHandler) {
	s.stopHandler = handler
}

// OnConnect 监听连接打开
// @param handler network.ConnectHandler 连接打开处理函数
func (s *server) OnConnect(handler network.ConnectHandler) {
	s.connectHandler = handler
}

// OnDisconnect 监听连接关闭
// @param handler network.DisconnectHandler 连接关闭处理函数
func (s *server) OnDisconnect(handler network.DisconnectHandler) {
	s.disconnectHandler = handler
}

// OnHeartbeat 监听心跳
// @param handler network.HeartbeatHandler 心跳处理函数
func (s *server) OnHeartbeat(handler network.HeartbeatHandler) {
	s.heartbeatHandler = handler
}

// OnReceive 监听接收到消息
// @param handler network.ReceiveHandler 消息接收处理函数
func (s *server) OnReceive(handler network.ReceiveHandler) {
	s.receiveHandler = handler
}

// init 初始化TCP服务器
// 解析TCP地址，按配置创建TLS或原生TCP监听器；若任一环节失败则回滚启动状态
// @return @1 error 已启动、证书加载失败或监听地址不合法时返回的错误
func (s *server) init() error {
	if s.listener != nil {
		return errors.ErrServerStarted
	}

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

	if s.opts.enableProxyProtocol {
		s.listener = &proxyproto.Listener{Listener: s.listener}
	}

	return nil
}

// serve 等待连接
// 循环接受TCP连接并分配到独立协程处理；对瞬时错误采用指数退避重试，服务器关闭时结束
func (s *server) serve(ln net.Listener) {
	var delay time.Duration

	for {
		conn, err := ln.Accept()
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}
				if max := 1 * time.Second; delay > max {
					delay = max
				}

				log.Warnf("tcp accept error: %v; retrying in %v", err, delay)
				time.Sleep(delay)
				continue
			}

			log.Warnf("tcp accept error: %v", err)
			break
		}

		delay = 0

		switch ccc := conn.(type) {
		case *proxyproto.Conn:
			switch cc := ccc.Raw().(type) {
			case *net.TCPConn:
				cc.SetNoDelay(true)
			case *tls.Conn:
				if c, ok := cc.NetConn().(*net.TCPConn); ok {
					c.SetNoDelay(true)
				}
			}
		case *tls.Conn:
			if c, ok := ccc.NetConn().(*net.TCPConn); ok {
				c.SetNoDelay(true)
			}
		default:
			if c, ok := conn.(*net.TCPConn); ok {
				c.SetNoDelay(true)
			}
		}

		if err = s.connMgr.allocateConn(conn); err != nil {
			log.Errorf("connection allocate error: %v", err)
			_ = conn.Close()
		}
	}

	_ = s.Stop()
}
