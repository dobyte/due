// Package grpc implements the microservice transport component on top of the gRPC protocol, and
// provides the creation of servers and clients, connection management and resource release.
package grpc

import (
	"sync"

	"github.com/dobyte/due/transport/grpc/v2/internal/client"
	"github.com/dobyte/due/transport/grpc/v2/internal/server"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
)

const name = "grpc"

// Transporter is a microservice transporter that creates servers and clients and manages the
// lifecycle of client connections.
type Transporter struct {
	opts    *options
	once    sync.Once
	builder *client.Builder
}

// NewTransporter returns a new transporter configured with the given options.
func NewTransporter(opts ...Option) *Transporter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Transporter{opts: o}
}

// Name returns the transporter component name.
func (t *Transporter) Name() string {
	return name
}

// SetDefaultDiscovery sets the default service discovery component.
func (t *Transporter) SetDefaultDiscovery(discovery registry.Discovery) {
	if t.opts.client.Discovery == nil {
		t.opts.client.Discovery = discovery
	}
}

// NewServer creates a microservice server.
func (t *Transporter) NewServer() (transport.Server, error) {
	return server.NewServer(&t.opts.server)
}

// NewClient creates a microservice client.
//
// The target may take one of the following forms:
//
//	direct://127.0.0.1:8011                         direct connection by address
//	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070   direct connection by instance ID
//	discovery://service_name                        service discovery by service name
func (t *Transporter) NewClient(target string) (transport.Client, error) {
	t.once.Do(func() {
		t.builder = client.NewBuilder(&t.opts.client)
	})

	cc, err := t.builder.Build(target)
	if err != nil {
		return nil, err
	}

	return client.NewClient(cc), nil
}

// Close closes the transporter and releases all client connections and resources.
func (t *Transporter) Close() error {
	if t.builder == nil {
		return nil
	}

	return t.builder.Close()
}
