package transport

import (
	"context"

	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/registry"
)

type Server interface {
	// Start starts the server.
	Start() error
	// Stop stops the server.
	Stop() error
	// Addr returns the listening address.
	Addr() string
	// Scheme returns the protocol scheme.
	Scheme() string
	// Endpoint returns the service endpoint.
	Endpoint() *endpoint.Endpoint
	// RegisterService registers a service.
	RegisterService(desc, service any) error
}

type Client interface {
	// Call invokes a service method.
	Call(ctx context.Context, service, method string, args any, reply any, opts ...any) error
	// Client returns the underlying client.
	Client() any
}

type Transporter interface {
	// Name returns the transporter component name.
	Name() string
	// NewServer creates a transport server.
	NewServer() (Server, error)
	// NewClient creates a transport client.
	//
	// The target may take one of the following forms:
	//
	//	direct://127.0.0.1:8011                         direct connection by address
	//	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070   direct connection by instance ID
	//	discovery://service_name                        service discovery by service name
	NewClient(target string) (Client, error)
	// SetDefaultDiscovery sets the default service discovery component.
	SetDefaultDiscovery(discovery registry.Discovery)
	// Close closes the transporter and releases all client connections and resources.
	Close() error
}

type NewMeshClient func(target string) (Client, error)
