package server

import (
	"context"
	"crypto/tls"

	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/errors"
	"github.com/smallnest/rpcx/server"
)

const scheme = "rpcx"

// Server 微服务服务器，封装 rpcx 服务端并暴露通用启动、停止与注册能力
type Server struct {
	listenAddr string
	exposeAddr string
	server     *server.Server
	endpoint   *endpoint.Endpoint
}

// Options 服务器配置项
type Options struct {
	Addr       string
	Expose     bool
	KeyFile    string
	CertFile   string
	ServerOpts []server.OptionFn
}

// NewServer 新建微服务服务器
// 解析监听与暴露地址，并按需启用 TLS 证书
// @param opts *Options 服务器配置项
// @return @1 *Server 服务器实例
// @return @2 error 错误信息
func NewServer(opts *Options) (*Server, error) {
	listenAddr, exposeAddr, err := net.ParseAddr(opts.Addr, opts.Expose)

	if err != nil {
		return nil, err
	}

	isSecure := false
	serverOpts := make([]server.OptionFn, 0)
	serverOpts = append(serverOpts, opts.ServerOpts...)
	if opts.CertFile != "" && opts.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, err
		}
		serverOpts = append(serverOpts, server.WithTLSConfig(&tls.Config{Certificates: []tls.Certificate{cert}}))
		isSecure = true
	}

	s := &Server{}
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.server = server.NewServer(serverOpts...)
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
// 以 TCP 协议在监听地址上提供 rpcx 服务
// @return @1 error 错误信息
func (s *Server) Start() error {
	return s.server.Serve("tcp", s.listenAddr)
}

// Stop 停止服务器
// @return @1 error 错误信息
func (s *Server) Stop() error {
	return s.server.Shutdown(context.Background())
}

// RegisterService 注册服务
// 服务描述符需为字符串形式的服务名
// @param desc any 服务描述符（服务名）
// @param ss any 服务实现实例
// @return @1 error 错误信息
func (s *Server) RegisterService(desc, ss any) error {
	name, ok := desc.(string)
	if !ok {
		return errors.ErrInvalidServiceDesc
	}

	return s.server.RegisterName(name, ss, "")
}
