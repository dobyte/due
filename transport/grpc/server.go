package grpc

import (
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/internal/xnet"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
)

type Server struct {
	err      error
	addr     string
	endpoint *endpoint.Endpoint
	lis      net.Listener
	server   *grpc.Server
}

func NewServer(opts ...Option) *Server {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	s := &Server{}

	host, port, err := net.SplitHostPort(o.addr)
	if err != nil {
		s.err = err
		return s
	}

	var (
		addr       string
		isSecure   bool
		serverOpts = make([]grpc.ServerOption, 0)
	)

	if len(host) > 0 && (host != "0.0.0.0" && host != "[::]" && host != "::") {
		s.addr = net.JoinHostPort(host, port)
		addr = s.addr
	} else {
		s.addr = net.JoinHostPort("", port)
		if ip, err := xnet.InternalIP(); err != nil {
			s.err = err
			return s
		} else {
			addr = net.JoinHostPort(ip, port)
		}
	}

	serverOpts = append(serverOpts, o.serverOpts...)

	if o.certFile != "" && o.keyFile != "" {
		cred, err := credentials.NewServerTLSFromFile(o.certFile, o.keyFile)
		if err != nil {
			s.err = err
			return s
		}
		serverOpts = append(serverOpts, grpc.Creds(cred))
		isSecure = true
	}

	s.server = grpc.NewServer(serverOpts...)
	s.endpoint = endpoint.NewEndpoint("grpc", addr, isSecure)

	return s
}

// Addr 监听地址
func (s *Server) Addr() string {
	return s.addr
}

// Scheme 协议
func (s *Server) Scheme() string {
	return s.endpoint.Scheme()
}

// Endpoint 获取服务端口
func (s *Server) Endpoint() *endpoint.Endpoint {
	return s.endpoint
}

// Start 启动服务器
func (s *Server) Start() error {
	if s.err != nil {
		return s.err
	}

	addr, err := net.ResolveTCPAddr("tcp", s.addr)
	if err != nil {
		return err
	}

	s.lis, err = net.Listen(addr.Network(), addr.String())
	if err != nil {
		return err
	}

	return s.server.Serve(s.lis)
}

// Stop 停止服务器
func (s *Server) Stop() error {
	if s.err != nil {
		return s.err
	}
	s.server.Stop()
	return s.lis.Close()
}

// RegisterService 注册服务
func (s *Server) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	s.server.RegisterService(sd, ss)
}
