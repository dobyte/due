package server

import (
	"errors"
	"fmt"
	"github.com/symsimmy/due/internal/endpoint"
	xnet "github.com/symsimmy/due/internal/net"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"net"
	"time"
)

const scheme = "grpc"

type Server struct {
	listenAddr       string
	exposeAddr       string
	endpoint         *endpoint.Endpoint
	server           *grpc.Server
	disabledServices []string
}

type Options struct {
	Addr                                          string
	HostAddr                                      string
	KeyFile                                       string
	CertFile                                      string
	KeepAliveEnforcementPolicyMinTime             int
	KeepAliveEnforcementPolicyPermitWithoutStream bool
	KeepAliveMaxConnectionIdle                    int
	KeepAliveMaxConnectionAge                     int
	KeepAliveMaxConnectionAgeGrace                int
	KeepAliveTime                                 int
	KeepAliveTimeout                              int
	ServerOpts                                    []grpc.ServerOption
}

func NewServer(opts *Options, disabledServices ...string) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr)
	if err != nil {
		return nil, err
	}

	if opts.HostAddr != "" {
		_, hostAddr, err := xnet.ParseAddr(opts.HostAddr)
		if err == nil {
			exposeAddr = hostAddr
		}
	}

	isSecure := false
	serverOpts := make([]grpc.ServerOption, 0, len(opts.ServerOpts)+3)
	serverOpts = append(serverOpts, opts.ServerOpts...)
	if opts.CertFile != "" && opts.KeyFile != "" {
		cred, err := credentials.NewServerTLSFromFile(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, err
		}
		serverOpts = append(serverOpts, grpc.Creds(cred))
		isSecure = true
	}

	if opts.KeepAliveTimeout > 0 && opts.KeepAliveTime > 0 {
		serverOpts = append(serverOpts, grpc.KeepaliveEnforcementPolicy(
			keepalive.EnforcementPolicy{
				MinTime:             time.Duration(opts.KeepAliveEnforcementPolicyMinTime) * time.Second, // If a client pings more than once every opts.KeepAliveEnforcementPolicyMinTime seconds, terminate the connection
				PermitWithoutStream: opts.KeepAliveEnforcementPolicyPermitWithoutStream,                  // Allow pings even when there are no active streams
			}))

		serverOpts = append(serverOpts, grpc.KeepaliveParams(
			keepalive.ServerParameters{
				MaxConnectionIdle:     time.Duration(opts.KeepAliveMaxConnectionIdle) * time.Second,     // If a client is idle for opts.KeepAliveMaxConnectionIdle seconds, send a GOAWAY
				MaxConnectionAge:      time.Duration(opts.KeepAliveMaxConnectionAge) * time.Second,      // If any connection is alive for more than opts.KeepAliveMaxConnectionAge seconds, send a GOAWAY
				MaxConnectionAgeGrace: time.Duration(opts.KeepAliveMaxConnectionAgeGrace) * time.Second, // Allow opts.KeepAliveMaxConnectionAgeGrace seconds for pending RPCs to complete before forcibly closing connections
				Time:                  time.Duration(opts.KeepAliveTime) * time.Second,                  // Ping the client if it is idle for opts.KeepAliveTime seconds to ensure the connection is still active
				Timeout:               time.Duration(opts.KeepAliveTimeout) * time.Second,               // Wait opts.KeepAliveTimeout second for the ping ack before assuming the connection is dead
			}))
	}

	s := &Server{}
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.server = grpc.NewServer(serverOpts...)
	s.endpoint = endpoint.NewEndpoint(scheme, exposeAddr, isSecure)
	s.disabledServices = disabledServices

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
func (s *Server) RegisterService(desc, service interface{}) error {
	sd, ok := desc.(*grpc.ServiceDesc)
	if !ok {
		return errors.New("invalid dispatcher desc")
	}

	for _, ds := range s.disabledServices {
		if ds == sd.ServiceName {
			return errors.New(fmt.Sprintf("unable to register %s dispatcher name", ds))
		}
	}

	s.server.RegisterService(sd, service)

	return nil
}
