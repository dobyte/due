package server

import (
	"net"

	"github.com/dobyte/due/v2/core/endpoint"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const scheme = "grpc"

type Server struct {
	listenAddr string
	exposeAddr string
	endpoint   *endpoint.Endpoint
	server     *grpc.Server
}

type Options struct {
	Addr       string
	Expose     bool
	KeyFile    string
	CertFile   string
	ServerOpts []grpc.ServerOption
}

func NewServer(opts *Options) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr)
	if err != nil {
		return nil, err
	}

	isSecure := false
	serverOpts := make([]grpc.ServerOption, 0, len(opts.ServerOpts)+2)
	serverOpts = append(serverOpts, opts.ServerOpts...)
	serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(recoverInterceptor))
	if opts.CertFile != "" && opts.KeyFile != "" {
		cred, err := credentials.NewServerTLSFromFile(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, err
		}
		serverOpts = append(serverOpts, grpc.Creds(cred))
		isSecure = true
	}

	s := &Server{}
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.server = grpc.NewServer(serverOpts...)
	s.endpoint = endpoint.NewEndpoint(scheme, exposeAddr, isSecure)

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

	listener, err := net.Listen(addr.Network(), addr.String())
	if err != nil {
		return err
	}

	return s.server.Serve(listener)
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.server.Stop()
	return nil
}

// RegisterService 注册服务
func (s *Server) RegisterService(desc, service any) error {
	switch sd := desc.(type) {
	case grpc.ServiceDesc:
		s.server.RegisterService(&sd, service)
	case *grpc.ServiceDesc:
		s.server.RegisterService(sd, service)
	default:
		return errors.ErrInvalidServiceDesc
	}

	return nil
}
