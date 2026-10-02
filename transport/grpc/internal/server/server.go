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

// Server is a microservice server that wraps a gRPC server and exposes generic start, stop and
// registration capabilities.
type Server struct {
	listenAddr string
	exposeAddr string
	endpoint   *endpoint.Endpoint
	server     *grpc.Server
}

// Options holds the server options.
type Options struct {
	Addr       string
	Expose     bool
	KeyFile    string
	CertFile   string
	ServerOpts []grpc.ServerOption
}

// NewServer returns a new microservice server.
//
// It parses the listen and expose addresses, and enables the TLS certificate and the unified
// recovery interceptor as needed.
func NewServer(opts *Options) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr)
	if err != nil {
		return nil, err
	}

	isSecure := false
	serverOpts := make([]grpc.ServerOption, 0, len(opts.ServerOpts)+2)
	serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(recoverInterceptor))
	serverOpts = append(serverOpts, opts.ServerOpts...)
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

// Addr returns the server listening address.
func (s *Server) Addr() string {
	return s.listenAddr
}

// Scheme returns the server protocol scheme.
func (s *Server) Scheme() string {
	return scheme
}

// Endpoint returns the service endpoint.
func (s *Server) Endpoint() *endpoint.Endpoint {
	return s.endpoint
}

// Start starts the server.
//
// It resolves the listening address and starts the gRPC service.
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

// Stop stops the server.
func (s *Server) Stop() error {
	s.server.GracefulStop()
	return nil
}

// RegisterService registers a service.
//
// It accepts a grpc.ServiceDesc or a pointer to one.
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
