package server

import (
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/log"
	"net"
	"sync"
	"time"
)

type Server struct {
	listener   net.Listener           // 监听器
	listenAddr string                 // 监听地址
	exposeAddr string                 // 暴露地址
	rw         sync.RWMutex           // 锁
	conns      map[net.Conn]*Conn     // 连接
	handlers   map[uint8]RouteHandler // 路由处理器
}

func NewServer(opts *Options) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr)
	if err != nil {
		return nil, err
	}

	s := &Server{}
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.conns = make(map[net.Conn]*Conn)
	s.handlers = make(map[uint8]RouteHandler)
	s.handlers[route.Handshake] = s.handshake

	return s, nil
}

// Addr 监听地址
func (s *Server) Addr() string {
	return s.exposeAddr
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

			log.Errorf("tcp accept connect error: %v", err)
			return nil
		}

		tempDelay = 0

		s.allocate(conn)
	}
}

// RegisterHandler 注册处理器
func (s *Server) RegisterHandler(route uint8, handler RouteHandler) {
	s.handlers[route] = handler
}

// 分配连接
func (s *Server) allocate(conn net.Conn) {
	s.rw.Lock()
	s.conns[conn] = newConn(s, conn)
	s.rw.Unlock()
}

// 回收连接
func (s *Server) recycle(conn net.Conn) {
	s.rw.Lock()
	delete(s.conns, conn)
	s.rw.Unlock()
}

// 处理握手
func (s *Server) handshake(conn *Conn, data []byte) error {
	seq, insKind, insID, err := protocol.DecodeHandshakeReq(data)
	if err != nil {
		return err
	}

	conn.InsKind = insKind
	conn.InsID = insID

	return conn.Send(protocol.EncodeBindRes(seq, codes.ErrorToCode(err)))
}
