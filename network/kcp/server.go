package kcp

import (
	"net"
	"sync"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/pires/go-proxyproto"
	"github.com/xtaci/kcp-go/v5"
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

// NewServer 创建服务器
// 按用户传入的选项覆盖默认配置，并初始化连接管理器
// @param opts ...ServerOption 服务器配置选项
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
// @return @1 string 服务器的监听地址
func (s *server) Addr() string {
	return s.opts.addr
}

// Start 启动服务器
// 初始化监听器后以协程方式接入连接，并触发启动hook函数
// @return @1 error 初始化失败时返回的错误
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
// 关闭监听器与全部连接，并触发关闭hook函数
// @return @1 error 服务器未运行或关闭监听器失败时返回的错误
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
// @return @1 string 协议名称"kcp"
func (s *server) Protocol() string {
	return protocol
}

// OnStart 监听服务器启动
// @param handler network.StartHandler 服务器启动hook函数
func (s *server) OnStart(handler network.StartHandler) {
	s.startHandler = handler
}

// OnStop 监听服务器关闭
// @param handler network.CloseHandler 服务器关闭hook函数
func (s *server) OnStop(handler network.CloseHandler) {
	s.stopHandler = handler
}

// OnConnect 监听连接打开
// @param handler network.ConnectHandler 连接打开hook函数
func (s *server) OnConnect(handler network.ConnectHandler) {
	s.connectHandler = handler
}

// OnDisconnect 监听连接关闭
// @param handler network.DisconnectHandler 连接关闭hook函数
func (s *server) OnDisconnect(handler network.DisconnectHandler) {
	s.disconnectHandler = handler
}

// OnHeartbeat 监听心跳
// @param handler network.HeartbeatHandler 心跳处理函数
func (s *server) OnHeartbeat(handler network.HeartbeatHandler) {
	s.heartbeatHandler = handler
}

// OnReceive 监听接收到消息
// @param handler network.ReceiveHandler 接收消息hook函数
func (s *server) OnReceive(handler network.ReceiveHandler) {
	s.receiveHandler = handler
}

// init 初始化服务器
// 创建KCP监听器并标记服务器为启动状态
// @return @1 error 服务器已启动或监听失败时返回的错误
func (s *server) init() error {
	if s.listener != nil {
		return errors.ErrServerStarted
	}

	addr, err := net.ResolveTCPAddr("tcp", s.opts.addr)
	if err != nil {
		return err
	}

	if s.listener, err = kcp.ListenWithOptions(addr.String(), nil, 0, 0); err != nil {
		return err
	}

	if s.opts.enableProxyProtocol {
		s.listener = &proxyproto.Listener{Listener: s.listener}
	}

	return nil
}

// serve 启动服务器
// 循环接受KCP连接并分配到连接管理器，监听结束时关闭全部连接
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

				log.Warnf("kcp accept error: %v; retrying in %v", err, delay)
				time.Sleep(delay)
				continue
			}

			if errors.Is(err, net.ErrClosed) {
				break
			}

			log.Warnf("kcp accept error: %v", err)
			break
		}

		delay = 0

		var session *kcp.UDPSession
		if pc, ok := conn.(*proxyproto.Conn); ok {
			session = pc.Raw().(*kcp.UDPSession)
		} else {
			session = conn.(*kcp.UDPSession)
		}

		if err = s.connMgr.allocateConn(session); err != nil {
			log.Errorf("connection allocate error: %v", err)
			_ = conn.Close()
		}
	}

	_ = s.Stop()
}
