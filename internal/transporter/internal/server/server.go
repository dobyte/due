package server

import (
	"net"
	"sync"
	"time"

	"github.com/dobyte/due/v2/core/endpoint"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/log"
)

const scheme = "drpc"

type Server struct {
	listener    net.Listener           // 监听器
	listenAddr  string                 // 监听地址
	exposeAddr  string                 // 暴露地址
	endpoint    *endpoint.Endpoint     // 暴露端点
	handlers    map[uint8]RouteHandler // 路由处理器
	rw          sync.RWMutex           // 锁
	connections map[net.Conn]*Conn     // 连接
}

func NewServer(opts *Options) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr)
	if err != nil {
		return nil, err
	}

	s := &Server{}
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.endpoint = endpoint.NewEndpoint(scheme, exposeAddr, false)
	s.connections = make(map[net.Conn]*Conn)
	s.handlers = make(map[uint8]RouteHandler)
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

// Start 启动服务器
func (s *Server) Start() error {
	addr, err := net.ResolveTCPAddr("tcp", s.listenAddr)
	if err != nil {
		return err
	}

	ln, err := net.ListenTCP(addr.Network(), addr)
	if err != nil {
		return err
	}

	s.listener = ln

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

				if tempDelay > time.Second {
					tempDelay = time.Second
				}

				log.Warnf("tcp accept connect error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}

			log.Warnf("tcp accept connect error: %v", err)
			return nil
		}

		tempDelay = 0

		s.allocate(conn)
	}
}

// Stop 停止服务器
func (s *Server) Stop() error {
	if err := s.listener.Close(); err != nil {
		return err
	}

	s.rw.Lock()
	for _, conn := range s.connections {
		_ = conn.close()
	}
	s.connections = nil
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

	return conn.Send(protocol.EncodeHandshakeRes(seq, codes.ErrorToCode(err)))
}
