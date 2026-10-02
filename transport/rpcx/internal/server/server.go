package server

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/errors"
	"github.com/smallnest/rpcx/server"
)

const scheme = "rpcx"

const defaultShutdownTimeout = 5 * time.Second

// Server is a microservice server that wraps an rpcx server and exposes generic start, stop and
// registration capabilities.
type Server struct {
	listenAddr string
	exposeAddr string
	server     *server.Server
	endpoint   *endpoint.Endpoint
}

// Options holds the server options.
type Options struct {
	Addr       string
	Expose     bool
	KeyFile    string
	CertFile   string
	ServerOpts []server.OptionFn
}

// NewServer returns a new microservice server.
//
// It parses the listen and expose addresses, and enables the TLS certificate as needed.
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
// It serves rpcx over TCP on the listening address.
func (s *Server) Start() error {
	return s.server.Serve("tcp", s.listenAddr)
}

// Stop stops the server.
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// RegisterService registers a service.
//
// The service descriptor must be a service name in string form.
func (s *Server) RegisterService(desc, ss any) error {
	name, ok := desc.(string)
	if !ok {
		return errors.ErrInvalidServiceDesc
	}

	return s.server.RegisterName(name, ss, "")
}
