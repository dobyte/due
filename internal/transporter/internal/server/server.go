package server

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/endpoint"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

const scheme = "drpc"

type RouteHandler func(conn *Conn, data []byte) error

type Server struct {
	opts        *Options
	listener    net.Listener       // 监听器
	listenAddr  string             // 监听地址
	exposeAddr  string             // 暴露地址
	endpoint    *endpoint.Endpoint // 暴露端点
	handlers    [256]RouteHandler  // 路由处理器
	started     atomic.Bool        // 是否已启动
	rw          sync.RWMutex       // 锁
	connections map[net.Conn]*Conn // 连接
}

func NewServer(opts *Options) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr, opts.Expose)
	if err != nil {
		return nil, err
	}

	s := &Server{}
	s.opts = opts
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.endpoint = endpoint.NewEndpoint(scheme, exposeAddr, false)
	s.connections = make(map[net.Conn]*Conn)
	s.handlers[route.Handshake] = s.handshake

	return s, nil
}

// Scheme 协议
func (s *Server) Scheme() string {
	return scheme
}

// ListenAddr 监听地址
func (s *Server) ListenAddr() string {
	return s.listenAddr
}

// ExposeAddr 暴露地址
func (s *Server) ExposeAddr() string {
	return s.exposeAddr
}

// Endpoint 暴露端点
func (s *Server) Endpoint() *endpoint.Endpoint {
	return s.endpoint
}

// init 初始化TCP服务器
// 解析TCP地址，按配置创建TLS或原生TCP监听器；若任一环节失败则回滚启动状态
// @return @1 error 已启动、证书加载失败或监听地址不合法时返回的错误
func (s *Server) init() error {
	if s.started.Swap(true) {
		return errors.ErrIllegalOperation
	}

	defer func() {
		if s.listener == nil {
			s.started.Store(false)
		}
	}()

	addr, err := net.ResolveTCPAddr("tcp", s.listenAddr)
	if err != nil {
		return err
	}

	ln, err := net.ListenTCP(addr.Network(), addr)
	if err != nil {
		return err
	}

	s.listener = ln

	return nil
}

// Start 启动服务器
func (s *Server) Start() error {
	if err := s.init(); err != nil {
		return err
	}

	xcall.Go(s.serve)

	return nil
}

// serve 等待连接
// 循环接受TCP连接并分配到独立协程处理；对瞬时错误采用指数退避重试，服务器关闭时结束
func (s *Server) serve() {
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

		s.allocate(conn)
	}

	s.Stop()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	if !s.started.CompareAndSwap(true, false) {
		return nil
	}

	if err := s.listener.Close(); err != nil {
		return err
	}

	s.listener = nil

	s.rw.Lock()
	for _, conn := range s.connections {
		_ = conn.close()
	}
	clear(s.connections)
	s.rw.Unlock()

	return nil
}

// RegisterHandler 注册处理器
func (s *Server) RegisterHandler(route uint8, handler RouteHandler) {
	s.handlers[route] = handler
}

// 分配连接
func (s *Server) allocate(conn net.Conn) {
	s.rw.Lock()
	s.connections[conn] = newConn(s, conn)
	s.rw.Unlock()
}

// 回收连接
func (s *Server) recycle(conn net.Conn) {
	s.rw.Lock()
	delete(s.connections, conn)
	s.rw.Unlock()
}

// 处理握手
func (s *Server) handshake(conn *Conn, data []byte) error {
	seq, insKind, insID, err := protocol.DecodeHandshakeReq(data)
	if err != nil {
		return err
	}

	conn.InsID = insID
	conn.InsKind = insKind

	return conn.Send(protocol.EncodeHandshakeRes(seq, codes.OK))
}
