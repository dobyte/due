package server

import (
	"fmt"
	"github.com/dobyte/due/errors"
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/utils/xnet"
	"github.com/smallnest/rpcx/server"
	"net"
)

const scheme = "grpc"

type Server struct {
	addr             string
	endpoint         *endpoint.Endpoint
	lis              net.Listener
	server           *server.Server
	excludedServices []string
}

type Options struct {
	Addr string
}

func NewServer(opts *Options) (*Server, error) {
	host, port, err := net.SplitHostPort(opts.Addr)
	if err != nil {
		return nil, err
	}

	s := &Server{}
	addr := ""
	if len(host) > 0 && (host != "0.0.0.0" && host != "[::]" && host != "::") {
		s.addr = net.JoinHostPort(host, port)
		addr = s.addr
	} else {
		s.addr = net.JoinHostPort("", port)
		if ip, err := xnet.InternalIP(); err != nil {
			return nil, err
		} else {
			addr = net.JoinHostPort(ip, port)
		}
	}

	s.server = server.NewServer()
	s.endpoint = endpoint.NewEndpoint(scheme, addr, false)

	return s, nil
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
	return s.server.Serve("tcp", s.addr)
}

// Stop 停止服务器
func (s *Server) Stop() error {
	return s.server.Close()
}

// RegisterService 注册服务
func (s *Server) RegisterService(desc, ss interface{}) error {
	name, ok := desc.(string)
	if !ok {
		return errors.New("invalid service desc")
	}

	for _, es := range s.excludedServices {
		if es == name {
			return errors.New(fmt.Sprintf("unable to register %s service name", es))
		}
	}

	return s.server.RegisterName(name, ss, "")
}

// RegisterSystemService 注册系统服务
func (s *Server) RegisterSystemService(name string, service interface{}, es []string) error {
	err := s.server.RegisterName(name, service, "")
	if err != nil {
		return err
	}

	s.excludedServices = es[:]

	return nil
}
