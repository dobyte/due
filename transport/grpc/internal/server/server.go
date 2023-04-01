package server

import (
	"errors"
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/utils/xnet"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
)

const scheme = "grpc"

type Server struct {
	addr     string
	endpoint *endpoint.Endpoint
	lis      net.Listener
	server   *grpc.Server
}

type Options struct {
	Addr       string
	KeyFile    string
	CertFile   string
	ServerOpts []grpc.ServerOption
}

func NewServer(opts *Options) (*Server, error) {
	host, port, err := net.SplitHostPort(opts.Addr)
	if err != nil {
		return nil, err
	}

	var (
		addr       string
		isSecure   = false
		serverOpts = make([]grpc.ServerOption, 0, len(opts.ServerOpts)+1)
		server     = &Server{}
	)

	if len(host) > 0 && (host != "0.0.0.0" && host != "[::]" && host != "::") {
		server.addr = net.JoinHostPort(host, port)
		addr = server.addr
	} else {
		server.addr = net.JoinHostPort("", port)
		if ip, err := xnet.InternalIP(); err != nil {
			return nil, err
		} else {
			addr = net.JoinHostPort(ip, port)
		}
	}

	serverOpts = append(serverOpts, opts.ServerOpts...)
	if opts.CertFile != "" && opts.KeyFile != "" {
		cred, err := credentials.NewServerTLSFromFile(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, err
		}
		serverOpts = append(serverOpts, grpc.Creds(cred))
		isSecure = true
	}

	server.server = grpc.NewServer(serverOpts...)
	server.endpoint = endpoint.NewEndpoint(scheme, addr, isSecure)

	return server, nil
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
	s.server.Stop()
	return s.lis.Close()
}

// RegisterService 注册服务
func (s *Server) RegisterService(desc, service interface{}) error {
	sd, ok := desc.(*grpc.ServiceDesc)
	if !ok {
		return errors.New("invalid service desc")
	}

	s.server.RegisterService(sd, service)

	return nil
}
