package quic

import (
	"context"
	"crypto/tls"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/quic-go/quic-go"
)

type serverRun struct {
	ctx      context.Context
	cancel   context.CancelFunc
	listener *quic.Listener
	manager  *serverConnMgr
	done     chan struct{}
	stopping bool
}

type server struct {
	opts              *serverOptions
	id                atomic.Int64
	mu                sync.Mutex
	run               *serverRun
	startHandler      network.StartHandler
	stopHandler       network.CloseHandler
	connectHandler    network.ConnectHandler
	disconnectHandler network.DisconnectHandler
	receiveHandler    network.ReceiveHandler
	heartbeatHandler  network.HeartbeatHandler
}

var _ network.Server = (*server)(nil)

// NewServer 创建一个QUIC服务器
// 须在启动前注册各类hook函数
// @param opts ...ServerOption 服务器配置项
// @return @1 network.Server 服务器实例
func NewServer(opts ...ServerOption) network.Server {
	o := defaultServerOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &server{opts: o}
}

// Addr 获取监听地址
// 服务器启动后返回监听器的实际地址，未启动时返回配置地址
// @return @1 string 监听地址
func (s *server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run != nil {
		return s.run.listener.Addr().String()
	}
	return s.opts.addr
}

// Start 启动服务器
// 每次调用开启全新的服务器生命周期，Stop 完成后可再次启动
// @return @1 error 错误信息
func (s *server) Start() error {
	s.mu.Lock()
	if s.run != nil {
		s.mu.Unlock()
		return errors.ErrIllegalOperation
	}
	cert, err := tls.LoadX509KeyPair(s.opts.certFile, s.opts.keyFile)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	config := transportConfig(s.opts.heartbeatInterval)
	config.HandshakeIdleTimeout = s.opts.handshakeTimeout
	ln, err := quic.ListenAddr(s.opts.addr, &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{alpn}}, config)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &serverRun{ctx: ctx, cancel: cancel, listener: ln, manager: newServerConnMgr(s), done: make(chan struct{})}
	s.run = r
	s.mu.Unlock()
	go s.serve(r)
	if s.startHandler != nil {
		s.startHandler()
	}
	return nil
}

// Stop 关闭服务器
// 停止接受新连接并关闭挂起与活跃的传输连接；
// 应用层回调可能在 Stop 返回后才执行完毕，因此回调中也可以调用 Stop
// @return @1 error 错误信息
func (s *server) Stop() error {
	s.mu.Lock()
	r := s.run
	if r == nil || r.stopping {
		s.mu.Unlock()
		return errors.ErrIllegalOperation
	}
	r.stopping = true
	r.cancel()
	s.mu.Unlock()
	err := r.listener.Close()
	r.manager.close()
	<-r.done
	return err
}

func (s *server) serve(r *serverRun) {
	var pending sync.WaitGroup
	for {
		qc, err := r.listener.Accept(r.ctx)
		if err != nil {
			if r.ctx.Err() == nil {
				log.Warnf("quic accept error: %v", err)
			}
			break
		}
		id, ok := r.manager.reserve(qc)
		if !ok {
			if r.manager.closed.Load() {
				log.Debugf("connection reserve failed: server is closing")
			} else {
				log.Errorf("connection reserve failed: connection limit reached")
			}
			_ = qc.CloseWithError(0, "connection limit reached")
			continue
		}
		pending.Add(1)
		go func() { defer pending.Done(); s.handleConn(r, id, qc) }()
	}
	r.cancel()
	_ = r.listener.Close()
	r.manager.close()
	pending.Wait()
	s.mu.Lock()
	if s.run == r {
		s.run = nil
	}
	close(r.done)
	s.mu.Unlock()
	if s.stopHandler != nil {
		s.stopHandler()
	}
}

func (s *server) handleConn(r *serverRun, id int64, qc *quic.Conn) {
	ctx, cancel := context.WithTimeout(r.ctx, s.opts.handshakeTimeout)
	defer cancel()
	stream, err := qc.AcceptStream(ctx)
	if err != nil {
		_ = qc.CloseWithError(0, "stream accept failed")
		r.manager.remove(id)
		return
	}
	if r.manager.allocateConn(id, qc, stream) == nil {
		log.Debugf("connection allocate failed: server is closing")
		_ = qc.CloseWithError(0, "server stopped")
		r.manager.remove(id)
		return
	}
}

// Protocol 获取协议名称
// @return @1 string 协议名称
func (s *server) Protocol() string { return protocol }

// OnStart 监听服务器启动
// 须在 Start 之前注册，Start 之后注册存在数据竞争
// @param h network.StartHandler 服务器启动处理函数
func (s *server) OnStart(h network.StartHandler) { s.startHandler = h }

// OnStop 监听服务器关闭
// 须在 Start 之前注册，Start 之后注册存在数据竞争
// @param h network.CloseHandler 服务器关闭处理函数
func (s *server) OnStop(h network.CloseHandler) { s.stopHandler = h }

// OnConnect 监听连接打开
// 须在 Start 之前注册，Start 之后注册存在数据竞争
// @param h network.ConnectHandler 连接打开处理函数
func (s *server) OnConnect(h network.ConnectHandler) { s.connectHandler = h }

// OnDisconnect 监听连接关闭
// 须在 Start 之前注册，Start 之后注册存在数据竞争
// @param h network.DisconnectHandler 连接关闭处理函数
func (s *server) OnDisconnect(h network.DisconnectHandler) { s.disconnectHandler = h }

// OnReceive 监听接收到消息
// 须在 Start 之前注册，Start 之后注册存在数据竞争；处理函数拥有每个接收缓冲的所有权
// @param h network.ReceiveHandler 消息接收处理函数
func (s *server) OnReceive(h network.ReceiveHandler) { s.receiveHandler = h }

// OnHeartbeat 监听心跳
// 须在 Start 之前注册，Start 之后注册存在数据竞争
// @param h network.HeartbeatHandler 心跳处理函数
func (s *server) OnHeartbeat(h network.HeartbeatHandler) { s.heartbeatHandler = h }
