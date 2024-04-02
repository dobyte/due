package server

import (
	"github.com/symsimmy/due/common/endpoint"
	xnet "github.com/symsimmy/due/common/net"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/transport/gnet/tcp"
)

const scheme = "tcp"

type Server struct {
	listenAddr string
	exposeAddr string
	endpoint   *endpoint.Endpoint
	server     *tcp.Server
}

type Options struct {
	Addr string
}

func (s *Server) RegisterService(desc, service interface{}) error {
	//TODO implement me
	panic("implement me")
}

func NewServer(opts *Options) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr)
	if err != nil {
		return nil, err
	}

	s := &Server{}
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.server = tcp.NewServer(opts.Addr)
	s.endpoint = endpoint.NewEndpoint(scheme, exposeAddr, false)

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
	s.server.Start()

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.server.Stop()
	return nil
}

// OnReceive 注册接收消息的回调
func (s *Server) OnReceive(handler transport.ReceiveHandler) {
	s.server.OnReceive(handler)
}
