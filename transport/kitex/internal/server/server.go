package server

import (
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/server"
	"github.com/dobyte/due/v2/core/endpoint"
	xnet "github.com/dobyte/due/v2/core/net"
	"net"
)

const scheme = "kitex"

type Options struct {
	Addr       string
	ServerOpts []server.Option
}

type Server struct {
	listenAddr string
	exposeAddr string
	endpoint   *endpoint.Endpoint
	server     server.Server
}

func NewServer(opts *Options) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr)
	if err != nil {
		return nil, err
	}

	addr, _ := net.ResolveTCPAddr("tcp", listenAddr)
	options := make([]server.Option, 0, len(opts.ServerOpts)+1)
	options = append(options, opts.ServerOpts...)
	//options = append(options, server.WithCompatibleMiddlewareForUnary())
	options = append(options, server.WithServiceAddr(addr))
	options = append(options, server.WithMuxTransport())

	s := &Server{}
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.server = server.NewServer(options...)
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

// RegisterService 注册服务
func (s *Server) RegisterService(desc, service interface{}) error {
	return s.server.RegisterService(desc.(*serviceinfo.ServiceInfo), service)
}

// Start 启动服务器
func (s *Server) Start() error {
	return s.server.Run()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	return s.server.Stop()
}
