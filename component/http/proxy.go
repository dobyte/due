package http

import (
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/transport"
	"github.com/gofiber/fiber/v3"
)

// Proxy is the HTTP proxy.
//
// It exposes the full set of features that an HTTP [Server] provides to the outside.
type Proxy struct {
	server *Server
}

// newProxy creates an HTTP proxy for s.
func newProxy(s *Server) *Proxy {
	return &Proxy{server: s}
}

// App returns the underlying fiber application.
func (p *Proxy) App() *fiber.App {
	return p.server.app
}

// Router returns the router.
func (p *Proxy) Router() Router {
	return &router{app: p.server.app, proxy: p}
}

// NewMeshClient creates a mesh client. The target may take one of three forms:
//   - direct connection: direct://127.0.0.1:8011
//   - direct connection: direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
//   - service discovery: discovery://service_name
func (p *Proxy) NewMeshClient(target string) (transport.Client, error) {
	if p.server.opts.transporter == nil {
		return nil, errors.ErrMissingTransporter
	}

	return p.server.opts.transporter.NewClient(target)
}
