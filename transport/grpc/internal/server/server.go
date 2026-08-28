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

// Server 微服务服务器，封装 gRPC 服务端并暴露通用启动、停止与注册能力
type Server struct {
	listenAddr string
	exposeAddr string
	endpoint   *endpoint.Endpoint
	server     *grpc.Server
}

// Options 服务器配置项
type Options struct {
	Addr       string
	Expose     bool
	KeyFile    string
	CertFile   string
	ServerOpts []grpc.ServerOption
}

// NewServer 新建微服务服务器
// 解析监听与暴露地址，并按需启用 TLS 证书与统一恢复拦截器
// @param opts *Options 服务器配置项
// @return @1 *Server 服务器实例
// @return @2 error 错误信息
func NewServer(opts *Options) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr)
	if err != nil {
		return nil, err
	}

	isSecure := false
	serverOpts := make([]grpc.ServerOption, 0, len(opts.ServerOpts)+2)
	serverOpts = append(serverOpts, opts.ServerOpts...)
	serverOpts = append(serverOpts, grpc.UnaryInterceptor(recoverInterceptor))
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

// Addr 获取服务器监听地址
// @return @1 string 监听地址
func (s *Server) Addr() string {
	return s.listenAddr
}

// Scheme 获取服务器协议
// @return @1 string 协议名称
func (s *Server) Scheme() string {
	return scheme
}

// Endpoint 获取服务端点
// @return @1 *endpoint.Endpoint 服务端点
func (s *Server) Endpoint() *endpoint.Endpoint {
	return s.endpoint
}

// Start 启动服务器
// 解析监听地址并启动 gRPC 服务
// @return @1 error 错误信息
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
// @return @1 error 错误信息
func (s *Server) Stop() error {
	s.server.GracefulStop()
	return nil
}

// RegisterService 注册服务
// 支持 grpc.ServiceDesc 或其指针形式
// @param desc any 服务描述符
// @param service any 服务实现实例
// @return @1 error 错误信息
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
