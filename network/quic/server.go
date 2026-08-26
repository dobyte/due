package quic

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/quic-go/quic-go"
)

type server struct {
	opts              *serverOptions            // 配置
	started           atomic.Bool               // 是否已启动
	mu                sync.Mutex                // 监听器锁
	listener          *quic.Listener            // 监听器
	ctx               context.Context           // 服务器上下文
	cancel            context.CancelFunc        // 取消函数
	connMgr           *serverConnMgr            // 连接管理器
	startHandler      network.StartHandler      // 服务器启动hook函数
	stopHandler       network.CloseHandler      // 服务器关闭hook函数
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
}

var _ network.Server = &server{}

// NewServer 创建一个QUIC服务器
// 基于 quic-go 实现，内部维护连接管理器（分片存储）、服务器上下文，并依赖配置中指定的证书/私钥用于TLS握手
// @param opts ...ServerOption 服务器配置项，可缺省，缺省时使用默认配置
// @return @1 network.Server 服务器实例
func NewServer(opts ...ServerOption) network.Server {
	o := defaultServerOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &server{}
	s.opts = o
	s.connMgr = newServerConnMgr(s)
	s.ctx, s.cancel = context.WithCancel(context.Background())

	return s
}

// Addr 监听地址
// @return @1 string 配置中的监听地址
func (s *server) Addr() string {
	return s.opts.addr
}

// Start 启动服务器
// 初始化监听器后在协程中开始等待连接，并触发启动成功钩子
// @return @1 error 启动失败（如已启动、证书加载失败或监听地址不合法）时返回的错误
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
// 取消上下文、关闭监听器并关闭所有连接，最后触发关闭钩子
// @return @1 error 关闭失败时返回的错误
func (s *server) Stop() error {
	if !s.started.Swap(false) {
		return errors.ErrIllegalOperation
	}

	s.cancel()

	s.mu.Lock()
	ln := s.listener
	s.listener = nil
	s.mu.Unlock()

	if ln != nil {
		if err := ln.Close(); err != nil {
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
// @return @1 string QUIC协议标识
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

// OnReceive 监听接收到消息
// @param handler network.ReceiveHandler 接收消息hook函数
func (s *server) OnReceive(handler network.ReceiveHandler) {
	s.receiveHandler = handler
}

// init 初始化QUIC服务器
// 构建TLS配置、解析UDP地址并创建QUIC监听器；若任一环节失败则回滚启动状态
// @return @1 error 已启动、证书加载失败或监听地址不合法时返回的错误
func (s *server) init() error {
	if s.started.Swap(true) {
		return errors.ErrIllegalOperation
	}

	defer func() {
		s.mu.Lock()
		if s.listener == nil {
			s.started.Store(false)
		}
		s.mu.Unlock()
	}()

	tlsConfig, err := s.buildTLSConfig()
	if err != nil {
		return err
	}

	udpAddr, err := net.ResolveUDPAddr("udp", s.opts.addr)
	if err != nil {
		return err
	}

	ln, err := quic.ListenAddr(udpAddr.String(), tlsConfig, &quic.Config{
		MaxIncomingStreams:   1000,
		MaxIdleTimeout:       s.opts.heartbeatInterval * 3,
		KeepAlivePeriod:      s.opts.heartbeatInterval / 2,
		HandshakeIdleTimeout: s.opts.handshakeTimeout,
		EnableDatagrams:      false,
		Allow0RTT:            false,
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	return nil
}

// buildTLSConfig 构建TLS配置
// 加载证书与私钥并设置QUIC协商协议（NextProtos）
// @return @1 *tls.Config TLS配置
// @return @2 error 证书或私钥加载失败时返回的错误
func (s *server) buildTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(s.opts.certFile, s.opts.keyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"due-quic"},
	}, nil
}

// serve 等待连接
// 循环接受QUIC连接并分配到独立协程处理；对瞬时错误采用指数退避重试，服务器关闭时结束并清理资源
func (s *server) serve() {
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()

	if listener == nil {
		return
	}

	var tempDelay time.Duration

	for {
		conn, err := listener.Accept(s.ctx)
		if err != nil {
			if errors.Is(err, quic.ErrServerClosed) || errors.Is(err, context.Canceled) {
				break
			}

			if tempDelay == 0 {
				tempDelay = 5 * time.Millisecond
			} else {
				tempDelay *= 2
			}
			if max := 1 * time.Second; tempDelay > max {
				tempDelay = max
			}

			log.Warnf("quic accept error: %v; retrying in %v", err, tempDelay)
			time.Sleep(tempDelay)
			continue
		}

		tempDelay = 0

		xcall.Go(func() {
			s.handleConn(conn)
		})
	}

	s.mu.Lock()
	s.listener = nil
	s.mu.Unlock()

	if s.started.CompareAndSwap(true, false) {
		s.connMgr.close()

		if s.stopHandler != nil {
			s.stopHandler()
		}
	}
}

// handleConn 处理连接
// 接受QUIC流并分配连接对象，分配失败时关闭流与连接
// @param qc *quic.Conn 已接受的QUIC连接
func (s *server) handleConn(qc *quic.Conn) {
	stream, err := qc.AcceptStream(s.ctx)
	if err != nil {
		log.Warnf("quic accept stream error: %v", err)
		_ = qc.CloseWithError(0, "stream accept failed")
		return
	}

	if err = s.connMgr.allocateConn(qc, stream); err != nil {
		log.Errorf("connection allocate error: %v", err)
		_ = stream.Close()
		_ = qc.CloseWithError(0, "connection allocate failed")
	}
}
