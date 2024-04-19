package server

import (
	"github.com/dobyte/due/v2/core/endpoint"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"net"
	"time"
)

const scheme = "drpc"

type Server struct {
	listener   net.Listener          // 监听器
	listenAddr string                // 监听地址
	exposeAddr string                // 暴露地址
	endpoint   *endpoint.Endpoint    // 暴露端点
	connMgr    *connMgr              // 连接管理器
	reader     *packet.Reader        // 数据读取器
	handlers   map[int8]RouteHandler // 路由处理器
}

type Options struct {
	Addr string
}

type RouteHandler func(conn *Conn, data []byte) error

func NewServer(opts *Options) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr)
	if err != nil {
		return nil, err
	}

	s := &Server{}
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.endpoint = endpoint.NewEndpoint(scheme, exposeAddr, false)
	s.connMgr = newConnMgr(s)
	s.reader = packet.NewReader()
	s.handlers = make(map[int8]RouteHandler)

	return s, nil
}

// Addr 监听地址
func (s *Server) Addr() string {
	return s.listenAddr
}

// Scheme 协议
func (s *Server) Scheme() string {
	return scheme
}

// Endpoint 获取服务端口
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
		cn, err := s.listener.Accept()
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

				log.Warnf("drpc tcp accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}

			log.Errorf("drpc accept error: %v", err)
			return nil
		}

		tempDelay = 0

		if err = s.connMgr.allocate(cn); err != nil {
			log.Errorf("connection allocate error: %v", err)
			_ = cn.Close()
		}
	}
}

func (s *Server) Stop() error {
	return nil
}

// RegisterHandler 注册处理器
func (s *Server) RegisterHandler(route int8, handler RouteHandler) {
	s.handlers[route] = handler
}

// RegisterService 注册服务
func (s *Server) RegisterService(desc, service interface{}) error {
	return nil
}
